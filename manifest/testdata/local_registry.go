package testdata

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/labs-brown-tape/attest/digest"
	"github.com/docker/labs-brown-tape/manifest/types"
	"github.com/docker/labs-brown-tape/trex"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-containerregistry/pkg/crane"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// RegistryCases keeps integration tests on a real, local TLS registry by default.
// Set TAPE_EXTERNAL_TESTS=1 to run the original public-image compatibility cases.
func RegistryCases(t *testing.T, pathToRoot string) TestCases {
	t.Helper()
	if os.Getenv("TAPE_EXTERNAL_TESTS") == "1" {
		return BaseYAMLCasesWithDigests(t)
	}
	trex.RunShared()
	opts := trex.Shared.CraneOptions()
	repoName := trex.Shared.NewUniqueRepoNamer("fixtures")
	names := []string{"first", "second"}
	refs := make([]string, len(names))
	expected := make([]types.Image, len(names))
	for i, name := range names {
		img, err := mutate.ConfigFile(empty.Image, &v1.ConfigFile{
			Architecture: "amd64", OS: "linux",
			Config: v1.Config{Labels: map[string]string{"fixture": name}},
		})
		if err != nil {
			t.Fatal(err)
		}
		// Include a real, tiny filesystem layer so copying exercises blob data,
		// not only image configuration and manifest metadata.
		var archive bytes.Buffer
		writer := tar.NewWriter(&archive)
		payload := []byte("Tape integration fixture: " + name + "\n")
		if err := writer.WriteHeader(&tar.Header{Name: "fixture.txt", Mode: 0644, Size: int64(len(payload))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		layer, err := tarball.LayerFromReader(bytes.NewReader(archive.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		img, err = mutate.AppendLayers(img, layer)
		if err != nil {
			t.Fatal(err)
		}
		repository := repoName(name)
		tagged := repository + ":v1"
		if err := crane.Push(img, tagged, opts...); err != nil {
			t.Fatal(err)
		}
		hash, err := img.Digest()
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = tagged
		// Cover both tag resolution and an already digest-pinned image.
		if i == 1 {
			refs[i] += "@" + hash.String()
		}
		alias := name
		expected[i] = types.Image{
			OriginalName: repository, OriginalTag: "v1", Digest: hash.String(), Alias: &alias,
		}
		// These are synthetic related-tag payloads, not valid signatures or SBOMs.
		for _, suffix := range []string{".sig", ".att", ".sbom"} {
			related := repository + ":" + strings.ReplaceAll(hash.String(), ":", "-") + suffix
			if err := crane.Push(empty.Image, related, opts...); err != nil {
				t.Fatal(err)
			}
		}
	}

	repoDir := t.TempDir()
	dir := filepath.Join(repoDir, "manifests")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := fmt.Sprintf("apiVersion: v1\nkind: Pod\nmetadata:\n  name: fixture\nspec:\n  containers:\n  - name: first\n    image: %s\n  - name: second\n    image: %s\n", refs[0], refs[1])
	if err := os.WriteFile(filepath.Join(dir, "app.yaml"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	// A small committed repository also exercises provenance without using or
	// changing the developer's worktree.
	repo, err := git.PlainInit(repoDir, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("manifests/app.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Commit("local test fixture", &git.CommitOptions{
		Author: &object.Signature{Name: "Tape tests", Email: "tests@example.invalid", When: time.Unix(1700000000, 0)},
	}); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(pathToRoot)
	if err != nil {
		t.Fatal(err)
	}
	relativeDir, err := filepath.Rel(root, dir)
	if err != nil {
		t.Fatal(err)
	}
	checksum := digest.SHA256(fmt.Sprintf("%x", sha256.Sum256([]byte(manifest))))
	lines := strings.Split(manifest, "\n")
	for i := range expected {
		line := 8 + i*2
		expected[i].Sources = []types.Source{{
			ImageSourceLocation: types.ImageSourceLocation{
				Manifest: "app.yaml", ManifestDigest: checksum,
				Line: line, Column: strings.Index(lines[line-1], refs[i]) + 1,
				NodePath: []string{"spec", "containers", "image"},
			},
			OriginalRef: refs[i],
		}}
	}
	return TestCases{{Description: "local-registry", Directory: relativeDir,
		Manifests: []string{"app.yaml"}, Expected: expected, NumRelatedTags: 6}}
}
