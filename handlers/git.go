package handlers

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/google/uuid"
)

type GitHandler struct {
	store   *store.Store
	appsDir string
	hub     *Hub
}

func NewGitHandler(s *store.Store, appsDir string, hub *Hub) *GitHandler {
	return &GitHandler{
		store:   s,
		appsDir: appsDir,
		hub:     hub,
	}
}

// authenticateRequest extracts Basic Auth credentials and validates JWT token
func (h *GitHandler) authenticateRequest(w http.ResponseWriter, r *http.Request) (userID string, ok bool) {
	_, password, hasAuth := r.BasicAuth()
	if !hasAuth {
		w.Header().Set("WWW-Authenticate", `Basic realm="Smack Git"`)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return "", false
	}

	// Password is the JWT token
	claims, err := middleware.ValidateToken(password)
	if err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="Smack Git"`)
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return "", false
	}

	return claims.UserID, true
}

// authorizeRead checks if user is a member of the app
func (h *GitHandler) authorizeRead(appID, userID string) error {
	isMember, err := h.store.IsAppMember(appID, userID)
	if err != nil || !isMember {
		return fmt.Errorf("not a member of this app")
	}
	return nil
}

// authorizeWrite checks if user is owner or admin
func (h *GitHandler) authorizeWrite(appID, userID string) error {
	role, err := h.store.GetAppMemberRole(appID, userID)
	if err != nil {
		return fmt.Errorf("not a member of this app")
	}
	if role != "owner" && role != "admin" {
		return fmt.Errorf("insufficient permissions: only owners and admins can push")
	}
	return nil
}

// validateAppID checks if the app ID is a valid UUID
func (h *GitHandler) validateAppID(appID string) bool {
	_, err := uuid.Parse(appID)
	return err == nil
}

// InfoRefs handles the git info/refs discovery request
func (h *GitHandler) InfoRefs(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")
	service := r.URL.Query().Get("service")

	if !h.validateAppID(appID) {
		http.Error(w, "Invalid app ID", http.StatusBadRequest)
		return
	}

	userID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	// Authorize based on service type
	switch service {
	case "git-upload-pack":
		if err := h.authorizeRead(appID, userID); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	case "git-receive-pack":
		if err := h.authorizeWrite(appID, userID); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	default:
		http.Error(w, "Invalid service", http.StatusBadRequest)
		return
	}

	// Ensure git repo exists and is synced with database
	repo, err := h.ensureGitRepo(appID)
	if err != nil {
		log.Printf("[Git] Failed to ensure repo for app %s: %v", appID, err)
		http.Error(w, "Failed to access repository", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")

	// Write packet line header
	pktLine := fmt.Sprintf("# service=%s\n", service)
	h.writePacketLine(w, pktLine)
	h.writeFlush(w)

	// Write refs
	h.writeRefs(w, repo, service)
}

// UploadPack handles git clone/fetch requests
func (h *GitHandler) UploadPack(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")

	if !h.validateAppID(appID) {
		http.Error(w, "Invalid app ID", http.StatusBadRequest)
		return
	}

	userID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	if err := h.authorizeRead(appID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	repoPath := filepath.Join(h.appsDir, appID, "git")
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-upload-pack-result")
	w.Header().Set("Cache-Control", "no-cache")

	// Parse the upload-pack request
	req := packp.NewUploadPackRequest()
	if err := req.Decode(r.Body); err != nil {
		log.Printf("[Git] Failed to decode upload-pack request: %v", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Get the wanted commits
	if len(req.Wants) == 0 {
		log.Printf("[Git] No wants in request")
		h.writePacketLine(w, "NAK\n")
		return
	}

	// Build the upload-pack response
	resp := packp.NewUploadPackResponse(req)

	// Generate packfile
	packData, err := h.generatePackfile(repo, req.Wants)
	if err != nil {
		log.Printf("[Git] Failed to generate packfile: %v", err)
		http.Error(w, "Failed to generate packfile", http.StatusInternalServerError)
		return
	}

	// Write NAK (no common commits with client)
	w.Write([]byte("0008NAK\n"))

	// Write packfile directly (without side-band for simplicity)
	w.Write(packData)

	_ = resp // We built our own response
}

// ReceivePack handles git push requests
func (h *GitHandler) ReceivePack(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appID")

	if !h.validateAppID(appID) {
		http.Error(w, "Invalid app ID", http.StatusBadRequest)
		return
	}

	userID, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	if err := h.authorizeWrite(appID, userID); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	repoPath := filepath.Join(h.appsDir, appID, "git")
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		http.Error(w, "Repository not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

	// Read the entire request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[Git] Failed to read request body: %v", err)
		h.sendReceivePackError(w, "Failed to read request")
		return
	}

	// Parse the command lines (before the packfile)
	// Format: old-sha1 new-sha1 refname\n
	commands, packStart := h.parseReceivePackCommands(body)
	if len(commands) == 0 {
		h.sendReceivePackError(w, "No commands in request")
		return
	}

	// Parse and apply the packfile
	if packStart < len(body) {
		packData := body[packStart:]
		if err := h.applyPackfile(repo, packData); err != nil {
			log.Printf("[Git] Failed to apply packfile: %v", err)
			h.sendReceivePackError(w, "Failed to apply packfile: "+err.Error())
			return
		}
	}

	// Update refs
	for _, cmd := range commands {
		if err := h.updateRef(repo, cmd); err != nil {
			log.Printf("[Git] Failed to update ref %s: %v", cmd.RefName, err)
			h.sendReceivePackError(w, "Failed to update ref: "+err.Error())
			return
		}
	}

	// Extract files from the new commit and update database
	if err := h.syncGitToDatabase(appID, repo); err != nil {
		log.Printf("[Git] Failed to sync to database: %v", err)
		h.sendReceivePackError(w, "Failed to sync files: "+err.Error())
		return
	}

	// Broadcast update via WebSocket
	h.broadcastCodeUpdate(appID)

	// Send success response
	h.sendReceivePackSuccess(w, commands)
}

type receivePackCommand struct {
	OldHash plumbing.Hash
	NewHash plumbing.Hash
	RefName string
}

func (h *GitHandler) parseReceivePackCommands(data []byte) ([]receivePackCommand, int) {
	var commands []receivePackCommand
	pos := 0

	for pos < len(data) {
		// Read packet length
		if pos+4 > len(data) {
			break
		}

		lenStr := string(data[pos : pos+4])
		var pktLen int
		fmt.Sscanf(lenStr, "%04x", &pktLen)

		if pktLen == 0 {
			// Flush packet - end of commands, packfile follows
			pos += 4
			break
		}

		if pos+pktLen > len(data) {
			break
		}

		line := string(data[pos+4 : pos+pktLen])
		line = strings.TrimSpace(line)

		// Remove capabilities (after first null byte)
		if idx := strings.IndexByte(line, 0); idx != -1 {
			line = line[:idx]
		}

		parts := strings.Fields(line)
		if len(parts) >= 3 {
			cmd := receivePackCommand{
				OldHash: plumbing.NewHash(parts[0]),
				NewHash: plumbing.NewHash(parts[1]),
				RefName: parts[2],
			}
			commands = append(commands, cmd)
		}

		pos += pktLen
	}

	return commands, pos
}

func (h *GitHandler) applyPackfile(repo *git.Repository, data []byte) error {
	if len(data) < 4 {
		return fmt.Errorf("packfile too short")
	}

	reader := bytes.NewReader(data)
	storer := repo.Storer

	// Use packfile scanner
	scanner := packfile.NewScanner(reader)

	// Read header
	_, objCount, err := scanner.Header()
	if err != nil {
		return fmt.Errorf("failed to read packfile header: %w", err)
	}

	log.Printf("[Git] Packfile contains %d objects", objCount)

	// Read each object
	for i := uint32(0); i < objCount; i++ {
		objHeader, err := scanner.NextObjectHeader()
		if err != nil {
			return fmt.Errorf("failed to read object header %d: %w", i, err)
		}

		// For delta objects, we need special handling
		if objHeader.Type == plumbing.OFSDeltaObject || objHeader.Type == plumbing.REFDeltaObject {
			return fmt.Errorf("delta objects not supported - client should send full objects")
		}

		// Create the object and write content directly to it
		obj := plumbing.MemoryObject{}
		obj.SetType(objHeader.Type)
		obj.SetSize(objHeader.Length)
		writer, err := obj.Writer()
		if err != nil {
			return fmt.Errorf("failed to create object writer: %w", err)
		}

		// Read object content using scanner's NextObject
		_, _, err = scanner.NextObject(writer)
		writer.Close()
		if err != nil {
			return fmt.Errorf("failed to read object content: %w", err)
		}

		hash, err := storer.SetEncodedObject(&obj)
		if err != nil {
			return fmt.Errorf("failed to store object: %w", err)
		}
		log.Printf("[Git] Stored object %s (type: %s, size: %d)", hash.String(), objHeader.Type, objHeader.Length)
	}

	return nil
}

func (h *GitHandler) updateRef(repo *git.Repository, cmd receivePackCommand) error {
	ref := plumbing.NewHashReference(plumbing.ReferenceName(cmd.RefName), cmd.NewHash)
	return repo.Storer.SetReference(ref)
}

func (h *GitHandler) sendReceivePackError(w http.ResponseWriter, msg string) {
	// Send unpack error
	h.writePacketLine(w, fmt.Sprintf("unpack %s\n", msg))
	h.writeFlush(w)
}

func (h *GitHandler) sendReceivePackSuccess(w http.ResponseWriter, commands []receivePackCommand) {
	h.writePacketLine(w, "unpack ok\n")
	for _, cmd := range commands {
		h.writePacketLine(w, fmt.Sprintf("ok %s\n", cmd.RefName))
	}
	h.writeFlush(w)
}

// ensureGitRepo creates or opens the git repo and syncs from database
func (h *GitHandler) ensureGitRepo(appID string) (*git.Repository, error) {
	repoPath := filepath.Join(h.appsDir, appID, "git")

	// Try to open existing repo
	repo, err := git.PlainOpen(repoPath)
	if err == nil {
		// Repo exists, return it as-is
		// Database-to-git sync only happens on: new repo init, or web UI code updates
		return repo, nil
	}

	// Create directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repo directory: %w", err)
	}

	// Init bare repository
	repo, err = git.PlainInit(repoPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to init repo: %w", err)
	}

	// Sync database content to repo
	if err := h.syncDatabaseToGit(appID, repo); err != nil {
		return nil, fmt.Errorf("failed to sync: %w", err)
	}

	return repo, nil
}

// syncDatabaseToGit creates git objects from app content
func (h *GitHandler) syncDatabaseToGit(appID string, repo *git.Repository) error {
	app, err := h.store.GetApp(appID)
	if err != nil {
		return err
	}

	storer := repo.Storer

	// Create blob for index.html
	htmlBlob := plumbing.MemoryObject{}
	htmlBlob.SetType(plumbing.BlobObject)
	htmlBlob.SetSize(int64(len(app.HTMLContent)))
	writer, _ := htmlBlob.Writer()
	writer.Write([]byte(app.HTMLContent))
	writer.Close()
	htmlHash, err := storer.SetEncodedObject(&htmlBlob)
	if err != nil {
		return fmt.Errorf("failed to store html blob: %w", err)
	}

	// Create blob for styles.css
	cssBlob := plumbing.MemoryObject{}
	cssBlob.SetType(plumbing.BlobObject)
	cssBlob.SetSize(int64(len(app.CSSContent)))
	writer, _ = cssBlob.Writer()
	writer.Write([]byte(app.CSSContent))
	writer.Close()
	cssHash, err := storer.SetEncodedObject(&cssBlob)
	if err != nil {
		return fmt.Errorf("failed to store css blob: %w", err)
	}

	// Create blob for script.js
	jsBlob := plumbing.MemoryObject{}
	jsBlob.SetType(plumbing.BlobObject)
	jsBlob.SetSize(int64(len(app.JSContent)))
	writer, _ = jsBlob.Writer()
	writer.Write([]byte(app.JSContent))
	writer.Close()
	jsHash, err := storer.SetEncodedObject(&jsBlob)
	if err != nil {
		return fmt.Errorf("failed to store js blob: %w", err)
	}

	// Create tree
	tree := object.Tree{
		Entries: []object.TreeEntry{
			{Name: "index.html", Mode: filemode.Regular, Hash: htmlHash},
			{Name: "script.js", Mode: filemode.Regular, Hash: jsHash},
			{Name: "styles.css", Mode: filemode.Regular, Hash: cssHash},
		},
	}

	treeObj := plumbing.MemoryObject{}
	if err := tree.Encode(&treeObj); err != nil {
		return fmt.Errorf("failed to encode tree: %w", err)
	}
	treeHash, err := storer.SetEncodedObject(&treeObj)
	if err != nil {
		return fmt.Errorf("failed to store tree: %w", err)
	}

	// Create commit
	commit := object.Commit{
		Author: object.Signature{
			Name:  "Smack",
			Email: "smack@example.com",
			When:  app.UpdatedAt,
		},
		Committer: object.Signature{
			Name:  "Smack",
			Email: "smack@example.com",
			When:  app.UpdatedAt,
		},
		Message:  fmt.Sprintf("Sync from database at %s", app.UpdatedAt.Format(time.RFC3339)),
		TreeHash: treeHash,
	}

	commitObj := plumbing.MemoryObject{}
	if err := commit.Encode(&commitObj); err != nil {
		return fmt.Errorf("failed to encode commit: %w", err)
	}
	commitHash, err := storer.SetEncodedObject(&commitObj)
	if err != nil {
		return fmt.Errorf("failed to store commit: %w", err)
	}

	// Update refs/heads/main
	mainRef := plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), commitHash)
	if err := storer.SetReference(mainRef); err != nil {
		return fmt.Errorf("failed to set main ref: %w", err)
	}

	// Update HEAD -> refs/heads/main
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, "refs/heads/main")
	if err := storer.SetReference(headRef); err != nil {
		return fmt.Errorf("failed to set HEAD: %w", err)
	}

	return nil
}

// syncGitToDatabase extracts files from HEAD commit and updates database
func (h *GitHandler) syncGitToDatabase(appID string, repo *git.Repository) error {
	ref, err := repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return fmt.Errorf("failed to get commit: %w", err)
	}

	tree, err := commit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	var htmlContent, cssContent, jsContent string

	for _, entry := range tree.Entries {
		blob, err := repo.BlobObject(entry.Hash)
		if err != nil {
			continue
		}
		reader, err := blob.Reader()
		if err != nil {
			continue
		}
		content, _ := io.ReadAll(reader)
		reader.Close()

		switch entry.Name {
		case "index.html":
			htmlContent = string(content)
		case "styles.css":
			cssContent = string(content)
		case "script.js":
			jsContent = string(content)
		}
	}

	return h.store.UpdateAppCode(appID, htmlContent, cssContent, jsContent)
}

// broadcastCodeUpdate notifies WebSocket clients of code changes
func (h *GitHandler) broadcastCodeUpdate(appID string) {
	app, err := h.store.GetApp(appID)
	if err != nil {
		return
	}

	if h.hub != nil {
		h.hub.BroadcastToApp(appID, models.WSMessage{
			Type: models.WSTypeAppCodeUpdated,
			Payload: models.AppCodeUpdatedPayload{
				AppID:       appID,
				HTMLContent: app.HTMLContent,
				CSSContent:  app.CSSContent,
				JSContent:   app.JSContent,
				UpdatedAt:   app.UpdatedAt.Format(time.RFC3339),
			},
		})
	}
}

// writePacketLine writes a pkt-line formatted string
func (h *GitHandler) writePacketLine(w io.Writer, s string) {
	length := len(s) + 4
	fmt.Fprintf(w, "%04x%s", length, s)
}

// writeFlush writes a flush packet (0000)
func (h *GitHandler) writeFlush(w io.Writer) {
	w.Write([]byte("0000"))
}

// writeRefs writes the refs advertisement
func (h *GitHandler) writeRefs(w io.Writer, repo *git.Repository, service string) {
	caps := h.getCapabilities(service)
	first := true

	// First, try to send HEAD
	head, err := repo.Head()
	if err == nil {
		line := fmt.Sprintf("%s HEAD\x00%s\n", head.Hash().String(), caps)
		h.writePacketLine(w, line)
		first = false
	}

	// Then send all non-symbolic refs (branches, tags)
	refs, err := repo.References()
	if err != nil {
		if first {
			// No refs at all, send fake ref with capabilities
			line := fmt.Sprintf("%s capabilities^{}\x00%s\n", plumbing.ZeroHash.String(), caps)
			h.writePacketLine(w, line)
		}
		h.writeFlush(w)
		return
	}

	refs.ForEach(func(ref *plumbing.Reference) error {
		// Skip symbolic references (like HEAD) - we already handled HEAD above
		if ref.Type() == plumbing.SymbolicReference {
			return nil
		}

		line := fmt.Sprintf("%s %s", ref.Hash().String(), ref.Name())
		if first {
			line = line + "\x00" + caps
			first = false
		}
		h.writePacketLine(w, line+"\n")
		return nil
	})

	// If no refs, send a fake ref with capabilities
	if first {
		line := fmt.Sprintf("%s capabilities^{}\x00%s\n", plumbing.ZeroHash.String(), caps)
		h.writePacketLine(w, line)
	}

	h.writeFlush(w)
}

func (h *GitHandler) getCapabilities(service string) string {
	if service == "git-upload-pack" {
		// For stateless HTTP, we need specific capabilities
		// no-done allows skipping the done packet, thin-pack for efficiency
		return "multi_ack_detailed no-done thin-pack ofs-delta shallow no-progress allow-tip-sha1-in-want"
	}
	// For receive-pack, don't advertise ofs-delta so client sends full objects
	return "report-status delete-refs no-thin"
}

// generatePackfile creates a packfile containing the requested objects
func (h *GitHandler) generatePackfile(repo *git.Repository, wants []plumbing.Hash) ([]byte, error) {
	// Collect all objects to include
	objects := make(map[plumbing.Hash]plumbing.EncodedObject)
	for _, want := range wants {
		if err := h.collectObjectsForPack(repo, want, objects); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer

	// Write pack header
	buf.Write([]byte("PACK"))    // Signature
	buf.WriteByte(0)             // Version (high byte)
	buf.WriteByte(0)             // Version
	buf.WriteByte(0)             // Version
	buf.WriteByte(2)             // Version = 2

	// Object count (4 bytes, big endian)
	count := len(objects)
	buf.WriteByte(byte(count >> 24))
	buf.WriteByte(byte(count >> 16))
	buf.WriteByte(byte(count >> 8))
	buf.WriteByte(byte(count))

	// Write each object
	for _, obj := range objects {
		if err := h.writePackObject(&buf, obj); err != nil {
			return nil, err
		}
	}

	// Calculate and write checksum (SHA-1 of everything so far)
	data := buf.Bytes()
	checksum := sha1.Sum(data)
	buf.Write(checksum[:])

	return buf.Bytes(), nil
}

func (h *GitHandler) collectObjectsForPack(repo *git.Repository, hash plumbing.Hash, objects map[plumbing.Hash]plumbing.EncodedObject) error {
	if _, exists := objects[hash]; exists {
		return nil
	}

	obj, err := repo.Storer.EncodedObject(plumbing.AnyObject, hash)
	if err != nil {
		return err
	}

	objects[hash] = obj

	switch obj.Type() {
	case plumbing.CommitObject:
		commit, err := repo.CommitObject(hash)
		if err != nil {
			return err
		}
		if err := h.collectObjectsForPack(repo, commit.TreeHash, objects); err != nil {
			return err
		}
		for _, parent := range commit.ParentHashes {
			h.collectObjectsForPack(repo, parent, objects)
		}

	case plumbing.TreeObject:
		tree, err := repo.TreeObject(hash)
		if err != nil {
			return err
		}
		for _, entry := range tree.Entries {
			if err := h.collectObjectsForPack(repo, entry.Hash, objects); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *GitHandler) writePackObject(buf *bytes.Buffer, obj plumbing.EncodedObject) error {
	// Read object content
	reader, err := obj.Reader()
	if err != nil {
		return err
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return err
	}

	size := len(content)
	objType := obj.Type()

	// Write object header
	// First byte: type (bits 4-6) and size (bits 0-3)
	typeNum := int(objType)
	b := byte((typeNum << 4) | (size & 0x0f))
	size >>= 4

	if size > 0 {
		b |= 0x80
	}
	buf.WriteByte(b)

	// Continue writing size bytes if needed
	for size > 0 {
		b = byte(size & 0x7f)
		size >>= 7
		if size > 0 {
			b |= 0x80
		}
		buf.WriteByte(b)
	}

	// Write compressed content
	zw := zlib.NewWriter(buf)
	zw.Write(content)
	zw.Close()

	return nil
}

// SyncDatabaseToGit is exported for use by AppsHandler when code is updated via web
func (h *GitHandler) SyncDatabaseToGit(appID string) error {
	repoPath := filepath.Join(h.appsDir, appID, "git")

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		// Repo doesn't exist yet, that's fine
		return nil
	}

	return h.syncDatabaseToGit(appID, repo)
}
