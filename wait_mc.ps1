Add-Type @"
using System;
using System.Runtime.InteropServices;
using System.Text;
public class Win {
  public delegate bool EnumDelegate(IntPtr hWnd, IntPtr lParam);
  [DllImport("user32.dll")] public static extern bool EnumWindows(EnumDelegate cb, IntPtr lParam);
  [DllImport("user32.dll")] public static extern int GetWindowText(IntPtr hWnd, StringBuilder sb, int n);
  [DllImport("user32.dll")] public static extern bool IsWindowVisible(IntPtr hWnd);
  [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT r);
  [StructLayout(LayoutKind.Sequential)] public struct RECT { public int L, T, R, B; }
  public static string OutPath = @"c:\Dev\Go\WaiLauncher\mc_poll.txt";
  public static bool CB(IntPtr hWnd, IntPtr lParam) {
    if (!IsWindowVisible(hWnd)) return true;
    var sb = new StringBuilder(256);
    GetWindowText(hWnd, sb, 256);
    if (sb.ToString().IndexOf("Minecraft", StringComparison.OrdinalIgnoreCase) >= 0) {
      RECT r; GetWindowRect(hWnd, out r);
      long ms = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();
      System.IO.File.AppendAllText(OutPath, string.Format("MC|{0}|L={1},T={2},W={3},H={4}|{5}\n", ms, r.L, r.T, r.R - r.L, r.B - r.T, sb.ToString()));
    }
    return true;
  }
}
"@
$out = "c:\Dev\Go\WaiLauncher\mc_poll.txt"
Set-Content -Path $out -Value ("POLLSTART|" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
$deadline = (Get-Date).AddSeconds(240)
while ((Get-Date) -lt $deadline) {
  $m = [Win].GetMethod('CB')
  $d = [System.Delegate]::CreateDelegate([Win+EnumDelegate], $m)
  [Win]::EnumWindows($d, [IntPtr]::Zero) | Out-Null
  Start-Sleep -Milliseconds 300
}
Add-Content -Path $out -Value ("POLLEND|" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())
