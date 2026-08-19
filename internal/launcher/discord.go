package launcher

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	goruntime "runtime"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// DiscordIPC implements a minimal Discord Rich Presence client speaking the
// official IPC protocol (no external DLLs). Presence is best-effort:
// when Discord is not running every call is a silent no-op.

type discordConn struct {
	c   io.ReadWriteCloser
	seq int
}

type discordPayload struct {
	Cmd   string          `json:"cmd"`
	Nonce string          `json:"nonce,omitempty"`
	Args  json.RawMessage `json:"args,omitempty"`
	Evt   string          `json:"evt,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
}

func ipcPipePath(idx int) string {
	if s, ok := os.LookupEnv("DISCORD_RPC_PIPE"); ok && s != "" {
		return s
	}
	if goruntime.GOOS == "windows" {
		return fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, idx)
	}
	tmp := os.TempDir()
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		tmp = xdg
	}
	return tmp + fmt.Sprintf("/discord-ipc-%d", idx)
}

func ipcDial(idx int) (io.ReadWriteCloser, error) {
	if goruntime.GOOS == "windows" {
		return os.OpenFile(ipcPipePath(idx), os.O_RDWR, 0)
	}
	return net.DialTimeout("unix", ipcPipePath(idx), 400*time.Millisecond)
}

func ipcConnect(clientID string) (*discordConn, error) {
	var conn io.ReadWriteCloser
	for idx := 0; idx < 10; idx++ {
		c, err := ipcDial(idx)
		if err == nil {
			conn = c
			break
		}
	}
	if conn == nil {
		return nil, fmt.Errorf("discord not reachable")
	}

	handshake, _ := json.Marshal(map[string]any{"v": 1, "client_id": clientID})
	if err := ipcWrite(conn, 0, handshake); err != nil {
		conn.Close()
		return nil, err
	}
	var resp discordPayload
	if _, err := ipcRead(conn, &resp); err != nil {
		conn.Close()
		return nil, err
	}
	if strings.ToUpper(resp.Cmd) != "DISPATCH" {
		conn.Close()
		return nil, fmt.Errorf("discord handshake rejected: %s", resp.Cmd)
	}
	return &discordConn{c: conn}, nil
}

func ipcWrite(w io.Writer, opcode uint32, data []byte) error {
	var hdr [8]byte
	binary.LittleEndian.PutUint32(hdr[0:4], opcode)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(data)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func ipcRead(r io.Reader, out *discordPayload) (uint32, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	length := binary.LittleEndian.Uint32(hdr[4:8])
	if length > 1<<20 {
		return 0, fmt.Errorf("ipc frame too large: %d", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, err
	}
	if out != nil {
		_ = json.Unmarshal(buf, out)
	}
	return op, nil
}

// DiscordRPC manages the Rich Presence session lifecycle.
type DiscordRPC struct {
	mu       sync.Mutex
	ClientID string // Discord Developer Portal application id; "" = disabled
	conn     *discordConn
	start    time.Time
}

// SetActivity shows "Playing Minecraft" with the build name and version.
func (d *DiscordRPC) SetActivity(buildName, version string) {
	if d.ClientID == "" {
		return
	}
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()

		if d.conn == nil {
			conn, err := ipcConnect(d.ClientID)
			if err != nil {
				return // Discord not running — silently skip
			}
			d.conn = conn
			d.start = time.Now()
		}

		activity := map[string]any{
			"details": buildName,
			"state":   "Minecraft " + version,
			"timestamps": map[string]any{
				"start": d.start.Unix(),
			},
			"assets": map[string]any{
				"large_image": "wailauncher",
				"large_text":  "WaiLauncher",
			},
		}
		args, _ := json.Marshal(map[string]any{
			"pid":      os.Getpid(),
			"activity": activity,
		})
		payload := discordPayload{
			Cmd:   "SET_ACTIVITY",
			Nonce: fmt.Sprintf("%d", time.Now().UnixNano()),
			Args:  args,
		}
		d.conn.seq++
		data, _ := json.Marshal(payload)
		if err := ipcWrite(d.conn.c, 1, data); err != nil {
			d.conn.c.Close()
			d.conn = nil
		}
	}()
}

// Clear removes the activity (called when the game exits).
func (d *DiscordRPC) Clear() {
	go func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		if d.conn == nil {
			return
		}
		args, _ := json.Marshal(map[string]any{"pid": os.Getpid()})
		payload := discordPayload{
			Cmd:   "SET_ACTIVITY",
			Nonce: fmt.Sprintf("%d", time.Now().UnixNano()),
			Args:  args,
		}
		data, _ := json.Marshal(payload)
		_ = ipcWrite(d.conn.c, 1, data)
		d.conn.c.Close()
		d.conn = nil
	}()
}
