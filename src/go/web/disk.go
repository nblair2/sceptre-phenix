package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"phenix/api/disk"
	"phenix/api/image"
	"phenix/util/common"
	"phenix/util/mm"
	"phenix/util/plog"
	"phenix/web/middleware"
	"phenix/web/rbac"
	"phenix/web/util"
)

// GetDisks - GET /disks.
func GetDisks(w http.ResponseWriter, r *http.Request) {
	plog.Debug(plog.TypeSystem, "HTTP handler called", "handler", "GetDisks")

	var (
		ctx             = r.Context()
		role, _         = ctx.Value(middleware.ContextKeyRole).(rbac.Role)
		query           = r.URL.Query()
		expName         = query.Get("expName")
		diskType        = query.Get("diskType")
		defaultDiskType = disk.VMImage | disk.ContainerImage | disk.ISOImage | disk.UNKNOWN
	)

	if !role.Allowed("disks", "list") {
		user, _ := ctx.Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"listing disks not allowed",
			"user",
			user,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	if len(diskType) > 0 {
		defaultDiskType = 0
		for s := range strings.SplitSeq(diskType, ",") {
			defaultDiskType |= disk.StringToKind(s)
		}
	}

	disks, err := disk.GetImages(expName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	filtered := []disk.Details{}

	for _, disk := range disks {
		if disk.Kind&defaultDiskType != 0 {
			filtered = append(filtered, disk)
		}
	}

	allowed := []disk.Details{}

	for _, disk := range filtered {
		if role.Allowed("disks", "list", disk.Name) {
			allowed = append(allowed, disk)
		}
	}

	sort.Slice(allowed, func(i, j int) bool {
		return allowed[i].Name < allowed[j].Name
	})

	body, err := json.Marshal(util.WithRoot("disks", allowed))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	_, _ = w.Write(body) //nolint:gosec // XSS via taint analysis
}

// CommitDisk - POST /disks/commit?disk={disk}.
func CommitDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]

	info, err := disk.GetImage(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if len(info.BackingImages) == 0 {
		http.Error(
			w,
			fmt.Sprintf("image %s has no backing image to commit to", path),
			http.StatusInternalServerError,
		)

		return
	}

	if !role.Allowed("disks", "update", info.BackingImages[0]) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"committing disk not allowed",
			"user",
			user,
			"from_disk",
			path,
			"to_disk",
			info.BackingImages[0],
		)
		http.Error(w, "forbidden for "+info.BackingImages[0], http.StatusForbidden)

		return
	}

	if !role.Allowed("disks", "update", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"committing disk not allowed",
			"user",
			user,
			"from_disk",
			path,
			"to_disk",
			info.BackingImages[0],
		)
		http.Error(w, "forbidden for "+path, http.StatusForbidden)

		return
	}

	err = disk.CommitDisk(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"committed disk",
		"user",
		user,
		"from_disk",
		path,
		"to_disk",
		info.BackingImages[0],
	)
	w.WriteHeader(http.StatusOK)
}

// SnapshotDisk - POST /disks/snapshot?disk={disk}&new={new}
// disk should be absolute
// new may be absolute, but will be put in same dir as disk if not. Extension will be set to qcow2.
//
//nolint:dupl // similar to RenameDisk
func SnapshotDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]
	newPath := normalizeDstDisk(path, mux.Vars(r)["new"])

	if !role.Allowed("disks", "create", filepath.Base(newPath)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"snapshotting disk not allowed",
			"user",
			user,
			"from_disk",
			path,
			"to_disk",
			newPath,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err := disk.SnapshotDisk(path, newPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"snapshotted disk",
		"user",
		user,
		"from_disk",
		path,
		"to_disk",
		newPath,
	)
	w.WriteHeader(http.StatusOK)
}

// RebaseDisk - POST /disks/rebase?disk={disk}&backing={backing}&unsafe={unsafe}
// disk and backing should be absolute.
func RebaseDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]
	backing := mux.Vars(r)["backing"]

	unsafe, err := strconv.ParseBool(mux.Vars(r)["unsafe"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if !role.Allowed("disks", "update", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"rebasing disk not allowed",
			"user",
			user,
			"disk",
			path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err = disk.RebaseDisk(path, backing, unsafe)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"rebased disk",
		"user",
		user,
		"disk",
		path,
		"onto",
		backing,
		"unsafe",
		unsafe,
	)
	w.WriteHeader(http.StatusOK)
}

// ResizeDisk - POST /disks/resize?disk={disk}&size={size}
// disk should be absolute. size should be a valid size (absolute or relative) per `qemu-img --help`.
func ResizeDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]
	size := mux.Vars(r)["size"]

	if !role.Allowed("disks", "update", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"resizing disk not allowed",
			"user",
			user,
			"disk",
			path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err := disk.ResizeDisk(path, size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"resized disk",
		"user",
		user,
		"disk",
		path,
		"size",
		size,
	)
	w.WriteHeader(http.StatusOK)
}

// CloneDisk - POST /disks/clone?disk={disk}&new={new}
// disk should be absolute
// new may be absolute, but will be put in same dir as disk if not. Extension will be set to qcow2.
func CloneDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]
	newPath := normalizeDstDisk(path, mux.Vars(r)["new"])

	if !role.Allowed("disks", "create") {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"cloning disk not allowed",
			"user",
			user,
			"from_disk",
			path,
			"to_disk",
			newPath,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err := disk.CloneDisk(path, newPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"cloned disk",
		"user",
		user,
		"from_disk",
		path,
		"to_disk",
		newPath,
	)
	w.WriteHeader(http.StatusOK)
}

// RenameDisk - POST /disks/rename?disk={disk}&new={new}
// disk should be absolute
// new may be absolute, but will be put in same dir as disk if not. Extension will be set to qcow2.
//
//nolint:dupl // similar to SnapshotDisk
func RenameDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]
	newPath := normalizeDstDisk(path, mux.Vars(r)["new"])

	if !role.Allowed("disks", "update", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"renaming disk not allowed",
			"user",
			user,
			"from_disk",
			path,
			"to_disk",
			newPath,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err := disk.RenameDisk(path, newPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"renamed disk",
		"user",
		user,
		"from_disk",
		path,
		"to_disk",
		newPath,
	)
	w.WriteHeader(http.StatusOK)
}

// DeleteDisk - DELETE /disks?disk={disk}
// disk should be absolute.
func DeleteDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]

	if !role.Allowed("disks", "delete", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"deleting disk not allowed",
			"user",
			user,
			"disk",
			path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	err := disk.DeleteDisk(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"deleted disk",
		"user",
		user,
		"disk",
		path,
	)
	w.WriteHeader(http.StatusOK)
}

// UploadDisk - POST /disks.
func UploadDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	clientFile, handler, err := r.FormFile("file")

	if !role.Allowed("disks", "upload") {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"uploading disk not allowed",
			"user",
			user,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	if err != nil {
		plog.Error(plog.TypeSystem, err.Error())
		http.Error(w, "Error uploading: "+err.Error(), http.StatusInternalServerError)
	}

	defer func() { _ = clientFile.Close() }()

	localFile, err := os.OpenFile( //nolint:gosec // Path traversal via taint analysis
		mm.GetMMFullPath(handler.Filename),
		os.O_WRONLY|os.O_CREATE,
		0o600,
	)
	if err != nil {
		plog.Error(plog.TypeSystem, err.Error())
		http.Error(w, "Error uploading: "+err.Error(), http.StatusInternalServerError)
	}

	defer func() { _ = localFile.Close() }()

	_, _ = io.Copy(localFile, clientFile)
	user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"uploaded disk",
		"user",
		user,
		"disk",
		localFile.Name(),
	)
}

// DownloadDisk - GET /disks?disk={disk}
// disk may be relative to filedir or absolute. If absolute must be in the files dir.
func DownloadDisk(w http.ResponseWriter, r *http.Request) {
	role, _ := r.Context().Value(middleware.ContextKeyRole).(rbac.Role)
	path := mux.Vars(r)["disk"]

	fileDir := mm.GetMMFilesDirectory()

	if !filepath.IsAbs(path) {
		path = filepath.Join(fileDir, path)
	} else if !strings.HasPrefix(path, fileDir) {
		errString := fmt.Sprintf("Error getting path %s: Path is not within files directory", path)
		plog.Error(plog.TypeSystem, errString)
		http.Error(w, errString, http.StatusBadRequest)

		return
	}

	if !role.Allowed("disks", "get", filepath.Base(path)) {
		user, _ := r.Context().Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"downloading disk not allowed",
			"user",
			user,
			"disk",
			path,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	fileInfo, err := os.Stat(path) //nolint:gosec // Path traversal via taint analysis
	if err != nil {
		errString := fmt.Sprintf("Error getting path %s: %s", path, err.Error())
		plog.Error(plog.TypeSystem, errString)
		http.Error(w, errString, http.StatusInternalServerError)

		return
	}

	if fileInfo.IsDir() {
		http.Error(w, "Can't download directory: "+path, http.StatusBadRequest)

		return
	}

	plog.Info(plog.TypeSystem, "download for file", "file", fileInfo.Name())

	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(fileInfo.Name()))
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, path)
}

// for output disk names - makes absolute and adds qcow2 file extension.
func normalizeDstDisk(src, dst string) string {
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(filepath.Dir(src), dst)
	}

	if !strings.HasSuffix(dst, ".qcow2") && !strings.HasSuffix(dst, ".qc2") {
		dst += ".qcow2"
	}

	return dst
}

type buildImageRequest struct {
	Verbosity int    `json:"verbosity"`
	Cache     bool   `json:"cache"`
	DryRun    bool   `json:"dry_run"`
	Output    string `json:"output"`
}

// BuildImage - POST /images/{name}/build.
// Asynchronously builds the named VM disk image using vmdb2.
func BuildImage(w http.ResponseWriter, r *http.Request) {
	plog.Debug(plog.TypeSystem, "HTTP handler called", "handler", "BuildImage")

	var (
		ctx     = r.Context()
		role, _ = ctx.Value(middleware.ContextKeyRole).(rbac.Role)
		vars    = mux.Vars(r)
		name    = vars["name"]
	)

	if !role.Allowed("images", "create", name) {
		user, _ := ctx.Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"building image not allowed",
			"user",
			user,
			"image",
			name,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		plog.Error(plog.TypeSystem, "reading request body", "err", err)
		http.Error(w, "unable to read request body", http.StatusInternalServerError)

		return
	}

	var req buildImageRequest

	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			plog.Error(plog.TypeSystem, "unmarshaling request body", "err", err)
			http.Error(w, "invalid request body", http.StatusBadRequest)

			return
		}
	}

	if req.Output == "" {
		req.Output = common.PhenixBase + "/images"
	}

	// Require absolute output paths to prevent directory traversal.
	if !filepath.IsAbs(req.Output) {
		http.Error(w, "output must be an absolute path", http.StatusBadRequest)

		return
	}

	req.Output = filepath.Clean(req.Output)

	user, _ := ctx.Value(middleware.ContextKeyUser).(string)

	plog.Info(
		plog.TypeAction,
		"image build started",
		"user",
		user,
		"image",
		name,
	)

	go func() {
		// Use a background context so the build continues after the HTTP request completes.
		buildCtx := context.Background()

		if err := image.Build(buildCtx, name, req.Verbosity, req.Cache, req.DryRun, req.Output); err != nil {
			plog.Error(plog.TypeSystem, "building image", "image", name, "err", err)
		} else {
			plog.Info(plog.TypeSystem, "image build complete", "image", name)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}

type injectMiniExeRequest struct {
	Exe  string `json:"exe"`
	Disk string `json:"disk"`
	SVC  string `json:"svc"`
}

// InjectMiniExe - POST /disks/inject.
// Injects miniccc/minirouter executable into the specified disk image.
func InjectMiniExe(w http.ResponseWriter, r *http.Request) {
	plog.Debug(plog.TypeSystem, "HTTP handler called", "handler", "InjectMiniExe")

	var (
		ctx     = r.Context()
		role, _ = ctx.Value(middleware.ContextKeyRole).(rbac.Role)
	)

	if !role.Allowed("disks", "create") {
		user, _ := ctx.Value(middleware.ContextKeyUser).(string)
		plog.Warn(
			plog.TypeSecurity,
			"injecting miniexe not allowed",
			"user",
			user,
		)
		http.Error(w, "forbidden", http.StatusForbidden)

		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		plog.Error(plog.TypeSystem, "reading request body", "err", err)
		http.Error(w, "unable to read request body", http.StatusInternalServerError)

		return
	}

	var req injectMiniExeRequest

	if err := json.Unmarshal(body, &req); err != nil {
		plog.Error(plog.TypeSystem, "unmarshaling request body", "err", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)

		return
	}

	if req.Exe == "" {
		http.Error(w, "exe path is required", http.StatusBadRequest)

		return
	}

	if req.Disk == "" {
		http.Error(w, "disk path is required", http.StatusBadRequest)

		return
	}

	// Require absolute paths to prevent directory traversal.
	if !filepath.IsAbs(req.Exe) {
		http.Error(w, "exe must be an absolute path", http.StatusBadRequest)

		return
	}

	if !filepath.IsAbs(req.Disk) {
		http.Error(w, "disk must be an absolute path", http.StatusBadRequest)

		return
	}

	// Clean paths to eliminate any traversal elements.
	req.Exe = filepath.Clean(req.Exe)
	req.Disk = filepath.Clean(req.Disk)

	if err := image.InjectMiniExe(req.Exe, req.Disk, req.SVC); err != nil {
		plog.Error(plog.TypeSystem, "injecting mini exe", "exe", req.Exe, "disk", req.Disk, "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	user, _ := ctx.Value(middleware.ContextKeyUser).(string)
	plog.Info(
		plog.TypeAction,
		"mini exe injected",
		"user",
		user,
		"exe",
		req.Exe,
		"disk",
		req.Disk,
	)
	w.WriteHeader(http.StatusNoContent)
}
