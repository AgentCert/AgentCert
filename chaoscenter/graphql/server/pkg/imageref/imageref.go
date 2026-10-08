// Package imageref resolves container image references against the image
// registry configured in .env, so ACE runs both inside a private network
// (every image from e.g. JFrog) and on the open internet (frozen copies on
// Docker Hub) without editing any chart, fault or workflow file.
//
// The naming rule is the same one implemented by scripts/lib/registry.sh,
// scripts/lib/resolve_image_env.py, the Helm "ace.image" helper and the
// install-app / install-agent post-renderers; scripts/tests/test-registry-tooling.sh
// checks that they all agree. Every image is written in the repo by its
// upstream public name:
//
//	IMAGE_REGISTRY set    -> <IMAGE_REGISTRY>/<public name>
//	                         (the public name keeps its own host, e.g.
//	                          <reg>/quay.io/containers/x:v1)
//	IMAGE_REGISTRY empty  -> <IMAGE_MIRROR_NAMESPACE>/<flat name> on Docker Hub
//	                         (default namespace "agentcert"; the flat name drops
//	                          the registry host and turns "/" into "-":
//	                          litmuschaos/k8s:latest -> agentcert/litmuschaos-k8s:latest)
//	                         — only for images that have a frozen copy (mirroredImages,
//	                         generated from deploy/images.txt); any other image keeps
//	                         its public name. IMAGE_MIRROR_NAMESPACE=none keeps every
//	                         public name.
//
// Resolution is idempotent: an already-resolved reference resolves to itself.
package imageref

import (
	"os"
	"regexp"
	"strings"
)

// DefaultMirrorNamespace is the Docker Hub namespace holding ACE's frozen
// copies of third-party images.
const DefaultMirrorNamespace = "agentcert"

// Resolver applies the naming rule for one registry configuration.
type Resolver struct {
	// Registry is IMAGE_REGISTRY, normalised to "host/path" (may be empty).
	Registry string
	// MirrorNamespace is IMAGE_MIRROR_NAMESPACE ("none" disables the frozen copies).
	MirrorNamespace string
}

// New returns a Resolver for the given IMAGE_REGISTRY / IMAGE_MIRROR_NAMESPACE values.
func New(registry, mirrorNamespace string) Resolver {
	ns := strings.TrimSpace(mirrorNamespace)
	if ns == "" {
		ns = DefaultMirrorNamespace
	}
	return Resolver{Registry: NormalizeRegistry(registry), MirrorNamespace: ns}
}

// FromEnv builds a Resolver from IMAGE_REGISTRY and IMAGE_MIRROR_NAMESPACE.
func FromEnv() Resolver {
	return New(os.Getenv("IMAGE_REGISTRY"), os.Getenv("IMAGE_MIRROR_NAMESPACE"))
}

// Active reports whether resolving changes anything at all (false only for
// IMAGE_REGISTRY empty + IMAGE_MIRROR_NAMESPACE=none).
func (r Resolver) Active() bool {
	return r.Registry != "" || r.MirrorNamespace != "none"
}

// NormalizeRegistry strips a scheme, the Artifactory UI path segment and any
// trailing slash, so pasted browser URLs work.
func NormalizeRegistry(value string) string {
	v := strings.TrimSpace(value)
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	v = strings.Replace(v, "/ui/native/", "/", 1)
	return strings.TrimRight(v, "/")
}

// Canonical drops an explicit docker.io / library prefix and adds ":latest"
// when the reference has neither a tag nor a digest.
func Canonical(ref string) string {
	ref = strings.TrimSpace(ref)
	for _, p := range []string{"docker.io/", "index.docker.io/", "registry-1.docker.io/"} {
		if strings.HasPrefix(ref, p) {
			ref = strings.TrimPrefix(ref, p)
			break
		}
	}
	ref = strings.TrimPrefix(ref, "library/")
	last := ref[strings.LastIndex(ref, "/")+1:]
	if !strings.Contains(last, ":") && !strings.Contains(ref, "@") {
		ref += ":latest"
	}
	return ref
}

// hasRegistryHost reports whether the first path component of name is a
// registry host (contains "." or ":" or is "localhost").
func hasRegistryHost(name string) bool {
	i := strings.Index(name, "/")
	if i < 0 {
		return false
	}
	first := name[:i]
	return strings.ContainsAny(first, ".:") || first == "localhost"
}

// FlatRef maps a canonical reference to its frozen copy under namespace.
func FlatRef(ref, namespace string) string {
	if strings.HasPrefix(ref, namespace+"/") {
		return ref
	}
	i := strings.LastIndex(ref, ":")
	name, tag := ref[:i], ref[i+1:]
	if hasRegistryHost(name) {
		name = name[strings.Index(name, "/")+1:]
	}
	return namespace + "/" + strings.ReplaceAll(name, "/", "-") + ":" + tag
}

// Resolve returns where ref is pulled from under this configuration.
func (r Resolver) Resolve(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "{{") || strings.Contains(ref, "$(") {
		return ref // empty or templated (Argo / shell) — nothing to resolve
	}
	if r.Registry != "" {
		if strings.HasPrefix(ref, r.Registry+"/") {
			return ref
		}
		return r.Registry + "/" + Canonical(ref)
	}
	c := Canonical(ref)
	if r.MirrorNamespace != "none" && IsMirrored(c) {
		return FlatRef(c, r.MirrorNamespace)
	}
	return c
}

// IsMirrored reports whether a canonical public reference has a frozen copy
// under IMAGE_MIRROR_NAMESPACE (deploy/images.txt "mirror" rows). Images
// without one — user-uploaded charts, custom experiment images — keep their
// upstream name on the open-source path instead of pointing at a copy that
// does not exist.
func IsMirrored(canonicalRef string) bool {
	return mirroredImages[canonicalRef]
}

// NormalizeSource maps an *_IMAGE_SOURCE value to "local" or "registry".
// "jfrog" and "dockerhub" are the legacy names for "registry"; empty returns def.
func NormalizeSource(value, def string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return def
	case "registry", "jfrog", "dockerhub":
		return "registry"
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

// SourceIsRegistry reports whether the component controlled by the env var
// sourceKey (e.g. "LITMUS_IMAGES_SOURCE") pulls its images from the registry at
// run time, as opposed to images side-loaded into the cluster ("local"), which
// keep their public names. def is used when the variable is unset.
func SourceIsRegistry(sourceKey, def string) bool {
	return NormalizeSource(os.Getenv(sourceKey), def) == "registry"
}

// PullSecretName returns the image-pull Secret name to attach to pods, or ""
// when no private registry is configured (public images need no login).
func PullSecretName() string {
	if NormalizeRegistry(os.Getenv("IMAGE_REGISTRY")) == "" {
		return ""
	}
	if s := strings.TrimSpace(os.Getenv("IMAGE_PULL_SECRET_NAME")); s != "" {
		return s
	}
	return "registry-pull"
}

// intentionallyBrokenImageVars are fault settings whose image must fail to pull;
// rewriting them to a registry copy would defeat the fault.
var intentionallyBrokenImageVars = map[string]bool{
	"INVALID_IMAGE":      true,
	"INVALID_ARCH_IMAGE": true,
}

var (
	imageLineRe  = regexp.MustCompile(`(?m)^(\s*(?:-\s+)?image:[ \t]*)(["']?)([^"'\s#{}]+)(["']?)`)
	imageEnvRe   = regexp.MustCompile(`(?m)^(\s*-?\s*name:[ \t]*["']?)([A-Z0-9_]*_IMAGE)(["']?[ \t]*\r?\n\s*value:[ \t]*)(["']?)([^"'\s#{}]+)(["']?)`)
	containersRe = regexp.MustCompile(`^(\s*)containers:\s*$`)
)

// RewriteText resolves every container image in a block of Kubernetes YAML
// text: "image:" lines and the values of "*_IMAGE" env settings (fault tools
// such as LIB_IMAGE, TC_IMAGE, DEBUG_IMAGE), except the intentionally broken
// INVALID_IMAGE / INVALID_ARCH_IMAGE. skip(ref) may exclude references (e.g.
// images side-loaded locally). Formatting and comments are left untouched.
func (r Resolver) RewriteText(text string, skip func(string) bool) string {
	if !r.Active() {
		return text
	}
	resolve := func(ref string) string {
		if skip != nil && skip(ref) {
			return ref
		}
		return r.Resolve(ref)
	}
	text = imageLineRe.ReplaceAllStringFunc(text, func(m string) string {
		p := imageLineRe.FindStringSubmatch(m)
		return p[1] + p[2] + resolve(p[3]) + p[4]
	})
	return imageEnvRe.ReplaceAllStringFunc(text, func(m string) string {
		p := imageEnvRe.FindStringSubmatch(m)
		if intentionallyBrokenImageVars[p[2]] {
			return m
		}
		return p[1] + p[2] + p[3] + p[4] + resolve(p[5]) + p[6]
	})
}

// AddPullSecretText inserts "imagePullSecrets: [{name: secret}]" into every
// pod spec of rendered YAML (before each "containers:" key), unless that pod
// spec already declares imagePullSecrets. No-op for an empty secret name.
func AddPullSecretText(text, secret string) string {
	if secret == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+8)
	for i, line := range lines {
		if m := containersRe.FindStringSubmatch(line); m != nil && !siblingKeyExists(lines, i, m[1], "imagePullSecrets:") {
			out = append(out, m[1]+"imagePullSecrets:", m[1]+"- name: "+secret)
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// siblingKeyExists reports whether a key with the given indentation exists in
// the same mapping as lines[at] (scanning up and down until the mapping ends).
func siblingKeyExists(lines []string, at int, indent, key string) bool {
	inMapping := func(l string) bool {
		t := strings.TrimSpace(l)
		return t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(l, indent) && len(l) > len(indent)
	}
	for _, step := range []int{-1, 1} {
		for j := at + step; j >= 0 && j < len(lines) && inMapping(lines[j]); j += step {
			l := lines[j]
			if strings.HasPrefix(l, indent+key) {
				return true
			}
			if strings.TrimSpace(l) == "---" {
				break
			}
		}
	}
	return false
}

// PostRenderEnv is the environment variable that switches an ACE binary into
// Helm post-renderer mode (see PostRender).
const PostRenderEnv = "ACE_HELM_POST_RENDER"

// PostRender is a Helm post-renderer: it rewrites every image in the rendered
// manifests and adds the pull secret to every pod. Configured by
// ACE_IMAGE_REGISTRY, ACE_IMAGE_MIRROR_NAMESPACE and ACE_IMAGE_PULL_SECRET.
func PostRender(manifests string) string {
	r := New(os.Getenv("ACE_IMAGE_REGISTRY"), os.Getenv("ACE_IMAGE_MIRROR_NAMESPACE"))
	out := r.RewriteText(manifests, nil)
	return AddPullSecretText(out, strings.TrimSpace(os.Getenv("ACE_IMAGE_PULL_SECRET")))
}

// PostRenderEnvVars returns the environment for a helm process that should run
// the post-renderer with this configuration (nil when nothing would change).
func PostRenderEnvVars(r Resolver, pullSecret string) []string {
	if !r.Active() && pullSecret == "" {
		return nil
	}
	return []string{
		PostRenderEnv + "=1",
		"ACE_IMAGE_REGISTRY=" + r.Registry,
		"ACE_IMAGE_MIRROR_NAMESPACE=" + r.MirrorNamespace,
		"ACE_IMAGE_PULL_SECRET=" + pullSecret,
	}
}

// HelmPostRenderer returns the extra helm arguments and environment that make
// this binary rewrite images as a Helm post-renderer, for charts whose images
// follow the *_IMAGE_SOURCE variable sourceKey (default def). An empty
// sourceKey means charts setup.sh never side-loads (user-uploaded charts), which
// always follow the registry. Both are nil when the images were side-loaded
// locally or nothing would change. The binary's main must call PostRender when
// PostRenderEnv=1 (graphql's server.go does).
func HelmPostRenderer(sourceKey, def string) (args, env []string) {
	if sourceKey != "" && !SourceIsRegistry(sourceKey, def) {
		return nil, nil
	}
	env = PostRenderEnvVars(FromEnv(), PullSecretName())
	if env == nil {
		return nil, nil
	}
	self, err := os.Executable()
	if err != nil {
		return nil, nil
	}
	return []string{"--post-renderer", self}, env
}
