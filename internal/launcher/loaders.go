package launcher

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	fabricMetaURL  = "https://meta.fabricmc.net"
	forgeMavenURL  = "https://maven.minecraftforge.net/"
	forgePromosURL = "https://files.minecraftforge.net/net/minecraftforge/forge/promotions_slim.json"
	neoMavenURL    = "https://maven.neoforged.net/releases/"
)

// MavenPath converts a maven coordinate group:artifact:version[:classifier][@ext]
// into a maven repository relative path.
func MavenPath(name string) (string, error) {
	ext := "jar"
	if i := strings.LastIndex(name, "@"); i >= 0 {
		if ext2 := name[i+1:]; ext2 != "" {
			ext = ext2
		}
		name = name[:i]
	}
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return "", fmt.Errorf("bad maven coordinate %q", name)
	}
	group, artifact, version := parts[0], parts[1], parts[2]
	classifier := ""
	if len(parts) >= 4 && parts[3] != "*" {
		classifier = parts[3]
	}
	file := artifact + "-" + version
	if classifier != "" {
		file += "-" + classifier
	}
	file += "." + ext
	return strings.ReplaceAll(group, ".", "/") + "/" + artifact + "/" + version + "/" + file, nil
}

// LoaderVersionEntry is one selectable modloader version in the UI.
type LoaderVersionEntry struct {
	Version string `json:"version"`
	Label   string `json:"label"` // "", "recommended", "latest"
}

// GetLoaderVersions returns the available versions of a modloader for the
// given Minecraft version, newest first.
func (l *Launcher) GetLoaderVersions(ctx context.Context, loader, mcVersion string) ([]LoaderVersionEntry, error) {
	switch loader {
	case "fabric":
		return l.fabricLoaderVersions(ctx, mcVersion)
	case "forge":
		return l.forgeLoaderVersions(ctx, mcVersion)
	case "neoforge":
		return l.neoforgeLoaderVersions(ctx, mcVersion)
	}
	return nil, fmt.Errorf("unknown loader %q", loader)
}

func (l *Launcher) fabricLoaderVersions(ctx context.Context, mcVersion string) ([]LoaderVersionEntry, error) {
	var raw []struct {
		Loader struct {
			Version string `json:"version"`
			Stable  bool   `json:"stable"`
		} `json:"loader"`
	}
	url := fabricMetaURL + "/v2/versions/loader/" + mcVersion
	if err := fetchJSON(ctx, url, &raw); err != nil {
		return nil, fmt.Errorf(l.T("err.loader_versions"), err)
	}
	out := make([]LoaderVersionEntry, 0, len(raw))
	for _, e := range raw {
		if e.Loader.Version == "" {
			continue
		}
		label := ""
		if !e.Loader.Stable {
			label = "beta"
		}
		out = append(out, LoaderVersionEntry{Version: e.Loader.Version, Label: label})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(l.T("err.loader_none"), "Fabric", mcVersion)
	}
	return out, nil // meta api already returns newest first
}

type mavenMetadata struct {
	XMLName    xml.Name `xml:"metadata"`
	Versioning struct {
		Versions struct {
			Version []string `xml:"version"`
		} `xml:"versions"`
	} `xml:"versioning"`
}

// forgeLoaderVersions lists forge builds for one MC version using the maven
// metadata, labelling recommended/latest from the promotions file.
func (l *Launcher) forgeLoaderVersions(ctx context.Context, mcVersion string) ([]LoaderVersionEntry, error) {
	var promos struct {
		Promos map[string]string `json:"promos"`
	}
	rec, latest := "", ""
	if err := fetchJSON(ctx, forgePromosURL, &promos); err == nil {
		rec = promos.Promos[mcVersion+"-recommended"]
		latest = promos.Promos[mcVersion+"-latest"]
	}

	var meta mavenMetadata
	if err := fetchXML(ctx, forgeMavenURL+"net/minecraftforge/forge/maven-metadata.xml", &meta); err != nil {
		// fall back to the promotions-only pair
		if rec == "" && latest == "" {
			return nil, fmt.Errorf(l.T("err.loader_versions"), err)
		}
	}
	prefix := mcVersion + "-"
	seen := make(map[string]bool)
	out := []LoaderVersionEntry{}
	add := func(v, label string) {
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, LoaderVersionEntry{Version: v, Label: label})
	}
	versions := meta.Versioning.Versions.Version
	for i := len(versions) - 1; i >= 0; i-- { // newest first
		v := versions[i]
		if !strings.HasPrefix(v, prefix) {
			continue
		}
		short := strings.TrimPrefix(v, prefix)
		label := ""
		switch short {
		case rec:
			label = "recommended"
		case latest:
			label = "latest"
		}
		add(short, label)
	}
	if len(out) == 0 {
		add(rec, "recommended")
		add(latest, "latest")
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(l.T("err.loader_none"), "Forge", mcVersion)
	}
	return out, nil
}

// neoVersionPrefix maps a Minecraft version to the NeoForge version prefix.
// Old scheme: 1.20.1 keeps its own; 1.a.b -> a.b., 1.a -> a.0.
// New scheme (26.2 and up): NeoForge versions start with the MC version.
func neoVersionPrefix(mcVersion string) string {
	if mcVersion == "1.20.1" {
		return "1.20.1-"
	}
	parts := strings.Split(mcVersion, ".")
	if len(parts) >= 1 && parts[0] != "1" {
		return mcVersion + "."
	}
	if len(parts) >= 3 {
		return parts[1] + "." + parts[2] + "."
	}
	if len(parts) == 2 {
		return parts[1] + ".0."
	}
	return mcVersion + "-"
}

func (l *Launcher) neoforgeLoaderVersions(ctx context.Context, mcVersion string) ([]LoaderVersionEntry, error) {
	var meta mavenMetadata
	if err := fetchXML(ctx, neoMavenURL+"net/neoforged/neoforge/maven-metadata.xml", &meta); err != nil {
		return nil, fmt.Errorf(l.T("err.loader_versions"), err)
	}
	prefix := neoVersionPrefix(mcVersion)
	out := []LoaderVersionEntry{}
	versions := meta.Versioning.Versions.Version
	for i := len(versions) - 1; i >= 0; i-- { // newest first
		v := versions[i]
		if strings.HasPrefix(v, prefix) {
			out = append(out, LoaderVersionEntry{Version: v})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf(l.T("err.loader_none"), "NeoForge", mcVersion)
	}
	return out, nil
}

func fetchXML(ctx context.Context, url string, out any) error {
	data, err := fetchBytes(ctx, url)
	if err != nil {
		return err
	}
	return xml.Unmarshal(data, out)
}

// PickDefaultLoaderVersion returns the version used when the user did not
// choose one explicitly (recommended, else newest).
func (l *Launcher) PickDefaultLoaderVersion(ctx context.Context, loader, mcVersion string) (string, error) {
	list, err := l.GetLoaderVersions(ctx, loader, mcVersion)
	if err != nil {
		return "", err
	}
	for _, e := range list {
		if e.Label == "recommended" {
			return e.Version, nil
		}
	}
	if len(list) == 0 {
		return "", fmt.Errorf(l.T("err.loader_none"), loader, mcVersion)
	}
	return list[0].Version, nil
}
