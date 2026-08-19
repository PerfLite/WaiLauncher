package launcher

import (
	"context"
	"fmt"
	"strings"
)

// ResolveLoaderVersion returns the merged version json for a modloader build,
// installing loader files on first use. The returned VersionJSON is a
// self-contained launchable profile stored under versions/<id>/.
func (l *Launcher) ResolveLoaderVersion(ctx context.Context, loader, loaderVersion, mcVersion string, emit func(ProgressEvent), onLog func(string)) (*VersionJSON, error) {
	if loaderVersion == "" {
		v, err := l.PickDefaultLoaderVersion(ctx, loader, mcVersion)
		if err != nil {
			return nil, err
		}
		loaderVersion = v
	}
	switch loader {
	case "fabric":
		return l.resolveFabric(ctx, mcVersion, loaderVersion, emit)
	case "forge", "neoforge":
		return l.installForgeLike(ctx, loader, mcVersion, loaderVersion, emit, onLog)
	}
	return nil, fmt.Errorf("unknown loader %q", loader)
}

// baseVersionJSON returns the vanilla version metadata for mcVersion
// (cached locally after the first download).
func (l *Launcher) baseVersionJSON(ctx context.Context, mcVersion string) (*VersionJSON, error) {
	m, err := l.GetManifest(ctx, false)
	if err != nil {
		return nil, fmt.Errorf(l.T("err.manifest"), err)
	}
	ref := l.FindVersion(m, mcVersion)
	if ref == nil {
		return nil, fmt.Errorf(l.T("err.not_found"), mcVersion)
	}
	v, err := l.GetVersionJSON(ctx, *ref)
	if err != nil {
		return nil, fmt.Errorf(l.T("err.meta"), err)
	}
	return v, nil
}

// fabricProfile is the loader profile served by the Fabric meta API.
type fabricProfile struct {
	ID           string `json:"id"`
	InheritsFrom string `json:"inheritsFrom"`
	MainClass    string `json:"mainClass"`
	Arguments    struct {
		Game []Arg `json:"game"`
		JVM  []Arg `json:"jvm"`
	} `json:"arguments"`
	Libraries []Library `json:"libraries"`
}

func (l *Launcher) resolveFabric(ctx context.Context, mcVersion, loaderVersion string, emit func(ProgressEvent)) (*VersionJSON, error) {
	emit(ProgressEvent{Stage: "loader", Message: "Fabric " + loaderVersion})
	var prof fabricProfile
	url := fabricMetaURL + "/v2/versions/loader/" + mcVersion + "/" + loaderVersion + "/profile/json"
	if err := fetchJSON(ctx, url, &prof); err != nil {
		return nil, fmt.Errorf(l.T("err.loader_versions"), err)
	}
	if prof.ID == "" {
		prof.ID = "fabric-loader-" + loaderVersion + "-" + mcVersion
	}

	// Already generated on a previous launch?
	if v, err := l.loadLocalVersion(prof.ID); err == nil {
		return v, nil
	}

	base, err := l.baseVersionJSON(ctx, mcVersion)
	if err != nil {
		return nil, err
	}
	merged := mergeLoaderProfile(base, prof.ID, prof.MainClass, prof.Arguments.Game, prof.Arguments.JVM, prof.Libraries)
	if err := l.saveLocalVersion(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// mergeLoaderProfile overlays loader settings onto the vanilla version json.
// JVM args of the loader are appended to vanilla's (keeping -cp ${classpath}
// and natives dir); loader game args go in front of the vanilla ones.
func mergeLoaderProfile(base *VersionJSON, id, mainClass string, gameArgs, jvmArgs []Arg, libs []Library) *VersionJSON {
	merged := *base
	merged.ID = id
	if mainClass != "" {
		merged.MainClass = mainClass
	}
	args := &Arguments{}
	if base.Arguments != nil {
		args.JVM = append(args.JVM, base.Arguments.JVM...)
		args.Game = append(args.Game, base.Arguments.Game...)
	}
	args.JVM = append(args.JVM, jvmArgs...)
	args.Game = append(append([]Arg{}, gameArgs...), args.Game...)
	merged.Arguments = args

	seen := make(map[string]bool)
	var mergedLibs []Library
	for _, lib := range libs {
		key := libMavenKey(lib.Name)
		if key == "" {
			key = lib.Name
		}
		if !seen[key] {
			seen[key] = true
			mergedLibs = append(mergedLibs, lib)
		}
	}
	for _, lib := range base.Libraries {
		key := libMavenKey(lib.Name)
		if key == "" {
			key = lib.Name
		}
		if !seen[key] {
			seen[key] = true
			mergedLibs = append(mergedLibs, lib)
		}
	}
	merged.Libraries = mergedLibs
	return &merged
}

func libMavenKey(name string) string {
	parts := strings.Split(name, ":")
	if len(parts) >= 4 {
		// group:artifact:version:classifier
		return parts[0] + ":" + parts[1] + ":" + parts[3]
	}
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return name
}
