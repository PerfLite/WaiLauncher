package launcher

import "encoding/json"

// VersionManifest is the top-level Mojang version manifest (piston-meta).
type VersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []VersionRef `json:"versions"`
}

// VersionRef is one entry of the manifest list.
type VersionRef struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // release | snapshot | old_beta | old_alpha
	URL         string `json:"url"`
	ReleaseTime string `json:"releaseTime"`
}

// Download describes a downloadable file with optional checksum.
type Download struct {
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
	URL  string `json:"url"`
	Path string `json:"path"`
}

// VersionJSON is the per-version metadata document.
type VersionJSON struct {
	ID                 string     `json:"id"`
	Type               string     `json:"type"`
	MainClass          string     `json:"mainClass"`
	Assets             string     `json:"assets"`
	MinecraftArguments string     `json:"minecraftArguments"` // legacy (<1.13)
	Arguments          *Arguments `json:"arguments"`
	AssetIndex         *Download  `json:"assetIndex"`
	JavaVersion        *JavaReq   `json:"javaVersion"`
	Downloads          struct {
		Client Download `json:"client"`
	} `json:"downloads"`
	Libraries []Library `json:"libraries"`
}

// JavaReq is the javaVersion block Mojang adds to modern version jsons.
type JavaReq struct {
	MajorVersion int    `json:"majorVersion"`
	Component    string `json:"component"`
}

// Arguments holds the modern (1.13+) argument lists.
type Arguments struct {
	Game []Arg `json:"game"`
	JVM  []Arg `json:"jvm"`
}

// Arg is either a plain string or a rule-gated object {rules, value}.
type Arg struct {
	Rules  []Rule
	Values []string
}

func (a *Arg) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		a.Values = []string{s}
		return nil
	}
	var obj struct {
		Rules []Rule          `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	a.Rules = obj.Rules
	var single string
	if err := json.Unmarshal(obj.Value, &single); err == nil {
		a.Values = []string{single}
		return nil
	}
	var many []string
	if err := json.Unmarshal(obj.Value, &many); err != nil {
		return err
	}
	a.Values = many
	return nil
}

// Rule gates an argument or library behind an OS/feature condition.
type Rule struct {
	Action   string          `json:"action"` // allow | disallow
	OS       *OSRule         `json:"os"`
	Features map[string]bool `json:"features"`
}

// OSRule matches against the current operating system / architecture.
type OSRule struct {
	Name string `json:"name"` // windows | osx | linux
	Arch string `json:"arch"` // x86
}

// Library is a dependency of the game, possibly with native classifiers.
type Library struct {
	Name      string `json:"name"`
	URL       string `json:"url"` // maven base url for entries without downloads
	Downloads struct {
		Artifact    *Download            `json:"artifact"`
		Classifiers map[string]*Download `json:"classifiers"`
	} `json:"downloads"`
	Rules   []Rule            `json:"rules"`
	Natives map[string]string `json:"natives"` // os name -> classifier template
}

// AssetIndex maps logical asset paths to content hashes.
type AssetIndex struct {
	Objects map[string]AssetObject `json:"objects"`
}

// AssetObject is one hashed file in the assets tree.
type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}
