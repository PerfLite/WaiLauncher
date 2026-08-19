package launcher

import "runtime"

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	default:
		return "linux"
	}
}

// osArch reports "x86" on 32-bit systems, otherwise "" (matches any arch rule).
func osArch() string {
	if runtime.GOARCH == "386" {
		return "x86"
	}
	return ""
}

// rulesAllow implements the Mojang rule evaluation: last matching rule wins,
// default is disallow when any rules are present.
func rulesAllow(rules []Rule, features map[string]bool) bool {
	if len(rules) == 0 {
		return true
	}
	allowed := false
	for _, r := range rules {
		if !osMatches(r.OS) {
			continue
		}
		if len(r.Features) > 0 {
			ok := true
			for k, v := range r.Features {
				if features[k] != v {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
		}
		allowed = r.Action == "allow"
	}
	return allowed
}

func osMatches(o *OSRule) bool {
	if o == nil {
		return true
	}
	if o.Name != "" && o.Name != osName() {
		return false
	}
	if o.Arch != "" && o.Arch != osArch() {
		return false
	}
	return true
}
