// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package ostree

import (
	"errors"
	"fmt"
	"path/filepath"
	"unsafe"
)

// #cgo pkg-config: ostree-1
// #include <glib.h>
// #include <ostree.h>
// #include "glibsupport.h"
import "C"

func convertGError(errC *C.GError) error {
	if errC == nil {
		return errors.New("nil GError")
	}

	err := errors.New(C.GoString((*C.char)(C._g_error_get_message(errC))))
	defer C.g_error_free(errC)
	return err
}

// WalkFunc is a function called by Walk() for each file
type WalkFunc func(path string) error

// Repo represents a local ostree repository
type Repo struct {
	path string
	ptr  unsafe.Pointer
}

// OpenRepo attempts to open the repo at the given path
func OpenRepo(path string) (*Repo, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}

	pathC := C.CString(path)
	defer C.free(unsafe.Pointer(pathC))

	repoPath := C.g_file_new_for_path(pathC)
	defer C.g_object_unref(C.gpointer(repoPath))

	repoC := C.ostree_repo_new(repoPath)
	if repoC == nil {
		return nil, errors.New("failed to open repository")
	}

	repo := &Repo{path, unsafe.Pointer(repoC)}

	var errC *C.GError
	if C.ostree_repo_open(repoC, nil, &errC) == C.FALSE {
		return nil, convertGError(errC)
	}

	return repo, nil
}

// native converts an ostree repo struct to its C equivalent
func (r *Repo) native() *C.OstreeRepo {
	if r.ptr == nil {
		return nil
	}
	return (*C.OstreeRepo)(r.ptr)
}

// Path returns the repository path
func (r *Repo) Path() string {
	return r.path
}

// GetObjectPath returns the path to the OSTree object passed as argument
func (r *Repo) GetObjectPath(objectName string) string {
	return filepath.Join(r.path, "objects", objectName[:2], objectName[2:])
}

// GetMode returns the repository mode
func (r *Repo) GetMode() (string, error) {
	if r.ptr == nil {
		return "", errors.New("repo not initialized")
	}

	mode := C.ostree_repo_get_mode(r.native())

	switch mode {
	case C.OSTREE_REPO_MODE_BARE:
		return "bare", nil
	case C.OSTREE_REPO_MODE_ARCHIVE:
		return "archive", nil
	case C.OSTREE_REPO_MODE_BARE_USER:
		return "bare-user", nil
	case C.OSTREE_REPO_MODE_BARE_USER_ONLY:
		return "bare-user-only", nil
	}

	return "", errors.New("unknown repository mode")
}

// ListRefs lists all the refs in the repository
func (r *Repo) ListRefs() ([]string, error) {
	if r.ptr == nil {
		return nil, errors.New("repo not initialized")
	}

	var refsC *C.GHashTable
	var errC *C.GError
	if C.ostree_repo_list_refs(r.native(), nil, &refsC, nil, &errC) == C.FALSE {
		return nil, convertGError(errC)
	}

	var iter C.GHashTableIter
	C.g_hash_table_iter_init(&iter, refsC)

	refs := []string{}

	var hkey C.gpointer
	var hvalue C.gpointer
	for C.g_hash_table_iter_next(&iter, &hkey, &hvalue) == C.TRUE {
		refs = append(refs, C.GoString(C._g_strdup(hkey)))
	}

	return refs, nil
}

// ListRevisions returns a dictionary whose keys are refs and values are the corresponding revisions
func (r *Repo) ListRevisions() (map[string]string, error) {
	if r.ptr == nil {
		return nil, errors.New("repo not initialized")
	}

	refs, err := r.ListRefs()
	if err != nil {
		return nil, err
	}

	revs := map[string]string{}

	for _, ref := range refs {
		rev, err := r.ResolveRev(ref)
		if err != nil {
			return nil, err
		}
		revs[ref] = rev
	}

	return revs, nil
}

// GetParentRev returns the revision of the parent commit, or an empty string if it doesn't have one
func (r *Repo) GetParentRev(rev string) (string, error) {
	if r.ptr == nil {
		return "", errors.New("repo not initialized")
	}

	var variantC *C.GVariant
	var errC *C.GError
	if C.ostree_repo_load_variant_if_exists(r.native(), C.OSTREE_OBJECT_TYPE_COMMIT, C.CString(rev), &variantC, &errC) == C.FALSE {
		return "", convertGError(errC)
	}
	if variantC == nil {
		return "", fmt.Errorf("commit %s doesn't exist", rev)
	}
	return C.GoString(C.ostree_commit_get_parent(variantC)), nil
}

// ResolveRev returns the revision corresponding to the specified branch
func (r *Repo) ResolveRev(branch string) (string, error) {
	if r.ptr == nil {
		return "", errors.New("repo not initialized")
	}

	var revC *C.char
	var errC *C.GError
	if C.ostree_repo_resolve_rev(r.native(), C.CString(branch), C.FALSE, &revC, &errC) == C.FALSE {
		return "", convertGError(errC)
	}

	return C.GoString(revC), nil
}

// TraverseCommit returns an hash table with all the reachable objects from
// the passed commit checksum, traversing maxDepth parent commits
func (r *Repo) TraverseCommit(rev string, maxDepth int) ([]string, error) {
	if r.ptr == nil {
		return nil, errors.New("repo not initialized")
	}

	revC := C.CString(rev)
	maxDepthC := C.int(maxDepth)

	var creachable *C.GHashTable
	var errC *C.GError
	if C.ostree_repo_traverse_commit(r.native(), revC, maxDepthC, &creachable, nil, &errC) == C.FALSE {
		return nil, convertGError(errC)
	}

	var iter C.GHashTableIter
	C.g_hash_table_iter_init(&iter, creachable)

	objects := []string{}

	var object *C.GVariant
	for C._g_hash_table_iter_next_variant(&iter, &object, nil) == C.TRUE {
		var cchecksum *C.char
		var cobjectType C.OstreeObjectType
		C._g_variant_get_su(object, &cchecksum, &cobjectType)

		objectNameC := C.ostree_object_to_string(cchecksum, cobjectType)
		objectName := C.GoString(objectNameC)

		if cobjectType == C.OSTREE_OBJECT_TYPE_FILE {
			if mode, _ := r.GetMode(); mode == "archive" {
				// Append z for archive repositories
				objects = append(objects, fmt.Sprintf("%sz", objectName))
				continue
			}
		}

		objects = append(objects, objectName)
	}

	return objects, nil
}

// Prune prunes the repository
func (r *Repo) Prune(noPrune, onlyRefs bool) (int, int, uint64, error) {
	if r.ptr == nil {
		return 0, 0, 0, errors.New("repo not initialized")
	}

	var flags C.OstreeRepoPruneFlags = C.OSTREE_REPO_PRUNE_FLAGS_NONE
	if noPrune {
		flags |= C.OSTREE_REPO_PRUNE_FLAGS_NO_PRUNE
	}
	if onlyRefs {
		flags |= C.OSTREE_REPO_PRUNE_FLAGS_REFS_ONLY
	}

	var total C.gint
	var pruned C.gint
	var size C.guint64
	var errC *C.GError
	if C.ostree_repo_prune(r.native(), flags, -1, &total, &pruned, &size, nil, &errC) == C.FALSE {
		return 0, 0, 0, convertGError(errC)
	}

	return int(total), int(pruned), uint64(size), nil
}

func (r *Repo) walkStart(root *C.GFile, path string, walkFn WalkFunc) error {
	f := C.g_file_resolve_relative_path(root, C.CString(path))
	defer C.g_object_unref(C.gpointer(f))

	var errC *C.GError
	opts := "standard::name,standard::type,standard::size,standard::is-symlink,standard::symlink-target,unix::device,unix::inode,unix::mode,unix::uid,unix::gid,unix::rdev"
	info := C.g_file_query_info(f, C.CString(opts), C.G_FILE_QUERY_INFO_NOFOLLOW_SYMLINKS, nil, &errC)
	if info == nil {
		return convertGError(errC)
	}

	if C.g_file_info_get_file_type(info) == C.G_FILE_TYPE_DIRECTORY {
		if _, err := r.walkRecurse(f, 1, walkFn); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repo) walkRecurse(root *C.GFile, depth int, walkFn WalkFunc) (bool, error) {
	var errC *C.GError
	opts := "standard::name,standard::type"
	enumerator := C.g_file_enumerate_children(root, C.CString(opts), C.G_FILE_QUERY_INFO_NOFOLLOW_SYMLINKS, nil, &errC)
	if enumerator == nil {
		return false, convertGError(errC)
	}
	defer C.g_object_unref(C.gpointer(enumerator))

	for {
		info := C.g_file_enumerator_next_file(enumerator, nil, nil)
		if info == nil {
			return false, nil
		}
		defer C.g_object_unref(C.gpointer(info))

		child := C.g_file_enumerator_get_child(enumerator, info)
		defer C.g_object_unref(C.gpointer(child))

		// Call function with the absolute path to the child
		pathC := C.g_file_get_path(child)
		if err := walkFn(C.GoString(pathC)); err != nil {
			return false, err
		}

		if C.g_file_info_get_file_type(info) == C.G_FILE_TYPE_DIRECTORY {
			result, err := r.walkRecurse(child, depth, walkFn)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
	}

	return true, nil
}

// Walk walks the path and execute walkFn for each file
func (r *Repo) Walk(rev, path string, walkFn WalkFunc) error {
	var root *C.GFile
	var commitC *C.char
	var errC *C.GError
	if C.ostree_repo_read_commit(r.native(), C.CString(rev), &root, &commitC, nil, &errC) == C.FALSE {
		return convertGError(errC)
	}

	if err := r.walkStart(root, path, walkFn); err != nil {
		return err
	}

	return nil
}

// Checkout checks out the specified path from the revision rev
func (r *Repo) Checkout(rev, path, destPath string) error {
	var root *C.GFile
	var commitC *C.char
	var errC *C.GError
	if C.ostree_repo_read_commit(r.native(), C.CString(rev), &root, &commitC, nil, &errC) == C.FALSE {
		return convertGError(errC)
	}

	subtree := C.g_file_resolve_relative_path(root, C.CString(path))

	opts := "standard::name,standard::type,standard::size,standard::is-symlink,standard::symlink-target,unix::device,unix::inode,unix::mode,unix::uid,unix::gid,unix::rdev"
	info := C.g_file_query_info(subtree, C.CString(opts), C.G_FILE_QUERY_INFO_NOFOLLOW_SYMLINKS, nil, &errC)
	if info == nil {
		return convertGError(errC)
	}

	dest := C.g_file_new_for_path(C.CString(destPath))
	if C.ostree_repo_checkout_tree(r.native(), C.OSTREE_REPO_CHECKOUT_MODE_USER, 0, dest, C._ostree_repo_file(subtree), info, nil, &errC) == C.FALSE {
		return convertGError(errC)
	}

	return nil
}

// SetRefImmediate points ref to checksum for the specified remote
func (r *Repo) SetRefImmediate(remote, ref, checksum string) error {
	if r.ptr == nil {
		return errors.New("repo not initialized")
	}

	var remoteC *C.char
	if remote != "" {
		remoteC = C.CString(remote)
	}

	var errC *C.GError
	if C.ostree_repo_set_ref_immediate(r.native(), remoteC, C.CString(ref), C.CString(checksum), nil, &errC) == C.FALSE {
		return convertGError(errC)
	}

	return nil
}

// RegenerateSummary updates the summary
func (r *Repo) RegenerateSummary() error {
	if r.ptr == nil {
		return errors.New("repo not initialized")
	}

	var errC *C.GError
	if C.ostree_repo_regenerate_summary(r.native(), nil, nil, &errC) == C.FALSE {
		return convertGError(errC)
	}

	return nil
}
