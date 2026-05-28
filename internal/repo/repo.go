// Package repo handles git operations for cloning and inspecting skill repositories.
package repo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
)

// SkillDir represents a skill directory in the repo.
type SkillDir struct {
	Name    string
	TreeSHA string
}

// Client handles git operations for a specific repo.
type Client struct {
	URL    string
	Branch string
}

// CloneShallow clones the repo with depth 1 to the given destination directory.
func (c *Client) CloneShallow(ctx context.Context, dest string) error {
	branchRef := plumbing.NewBranchReferenceName(c.Branch)
	_, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:           c.URL,
		ReferenceName: branchRef,
		Depth:         1,
		SingleBranch:  true,
	})
	if err != nil {
		return fmt.Errorf("cloning repo: %w", err)
	}
	return nil
}

// ListSkillDirs opens a cloned repo and reads the root tree to find skill
// directories. Returns only top-level tree entries (subdirectories).
func (c *Client) ListSkillDirs(ctx context.Context, clonePath string) ([]SkillDir, error) {
	r, err := git.PlainOpen(clonePath)
	if err != nil {
		return nil, fmt.Errorf("opening repo: %w", err)
	}

	head, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("getting HEAD: %w", err)
	}

	commit, err := r.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("getting commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("getting tree: %w", err)
	}

	var skills []SkillDir
	for _, entry := range tree.Entries {
		// Skip regular files and executables — we only care about directories
		if entry.Mode != filemode.Dir {
			continue
		}
		// Skip hidden entries
		if strings.HasPrefix(entry.Name, ".") {
			continue
		}

		skills = append(skills, SkillDir{
			Name:    entry.Name,
			TreeSHA: entry.Hash.String(),
		})
	}

	return skills, nil
}

// ComputeLocalTreeSHA computes a deterministic SHA256 hash for a local folder.
// It walks all files and directories, sorts them by path, and recursively
// hashes content. The algorithm is deterministic: same folder contents always
// produce the same hash.
func ComputeLocalTreeSHA(root string) (string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", root)
	}

	hash, err := computeDirHash(root)
	if err != nil {
		return "", fmt.Errorf("computing tree sha for %s: %w", root, err)
	}
	return hex.EncodeToString(hash), nil
}

// computeDirHash recursively computes a SHA256 hash for a directory.
// Hidden files and directories (starting with ".") are excluded.
func computeDirHash(dir string) ([]byte, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	// Sort entries for deterministic ordering
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	h := sha256.New()
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(dir, name)

		if entry.IsDir() {
			subHash, err := computeDirHash(fullPath)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(h, "dir:%s:%x\n", name, subHash)
		} else {
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, err
			}
			contentHash := sha256.Sum256(data)
			fmt.Fprintf(h, "file:%s:%d:%x\n", name, len(data), contentHash)
		}
	}

	return h.Sum(nil), nil
}

