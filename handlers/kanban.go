package handlers

import (
	"encoding/json"
	"net/http"
	"smack-server/middleware"
	"smack-server/models"
	"smack-server/store"
	"time"
)

type KanbanHandler struct {
	store *store.Store
}

func NewKanbanHandler(s *store.Store) *KanbanHandler {
	return &KanbanHandler{store: s}
}

// Board handlers

func (h *KanbanHandler) ListBoards(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	boards, err := h.store.GetBoardsForUser(userID)
	if err != nil {
		http.Error(w, "Failed to fetch boards", http.StatusInternalServerError)
		return
	}

	if boards == nil {
		boards = []models.BoardWithDetails{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(boards)
}

func (h *KanbanHandler) CreateBoard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)

	var req models.CreateBoardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Board name is required", http.StatusBadRequest)
		return
	}

	board, err := h.store.CreateBoard(req.Name, req.Description, req.Icon, userID)
	if err != nil {
		http.Error(w, "Failed to create board", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(board)
}

func (h *KanbanHandler) GetBoard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	// Check membership
	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	// Get board with columns and cards
	board, err := h.store.GetBoard(boardID)
	if err != nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}

	columns, err := h.store.GetColumnsWithCards(boardID)
	if err != nil {
		http.Error(w, "Failed to fetch board data", http.StatusInternalServerError)
		return
	}

	labels, err := h.store.GetLabelsForBoard(boardID)
	if err != nil {
		labels = []models.KanbanLabel{}
	}

	response := struct {
		*models.Board
		Columns []models.KanbanColumnWithCards `json:"columns"`
		Labels  []models.KanbanLabel           `json:"labels"`
	}{
		Board:   board,
		Columns: columns,
		Labels:  labels,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *KanbanHandler) UpdateBoard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	// Check membership
	role, err := h.store.GetBoardMemberRole(boardID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to update this board", http.StatusForbidden)
		return
	}

	var req models.UpdateBoardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	board, err := h.store.GetBoard(boardID)
	if err != nil {
		http.Error(w, "Board not found", http.StatusNotFound)
		return
	}

	name := board.Name
	description := board.Description
	if req.Name != "" {
		name = req.Name
	}
	if req.Description != "" {
		description = req.Description
	}

	err = h.store.UpdateBoard(boardID, name, description, req.Icon)
	if err != nil {
		http.Error(w, "Failed to update board", http.StatusInternalServerError)
		return
	}

	board, _ = h.store.GetBoard(boardID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(board)
}

func (h *KanbanHandler) DeleteBoard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	// Only owner can delete
	role, err := h.store.GetBoardMemberRole(boardID, userID)
	if err != nil || role != "owner" {
		http.Error(w, "Only board owner can delete", http.StatusForbidden)
		return
	}

	err = h.store.DeleteBoard(boardID)
	if err != nil {
		http.Error(w, "Failed to delete board", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// Board member handlers

func (h *KanbanHandler) GetBoardMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	members, err := h.store.GetBoardMembers(boardID)
	if err != nil {
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}

func (h *KanbanHandler) AddBoardMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	// Check if user can add members
	role, err := h.store.GetBoardMemberRole(boardID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to add members", http.StatusForbidden)
		return
	}

	var req models.AddBoardMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	memberRole := req.Role
	if memberRole == "" {
		memberRole = "member"
	}

	err = h.store.AddBoardMember(boardID, req.UserID, memberRole)
	if err != nil {
		http.Error(w, "Failed to add member", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

func (h *KanbanHandler) RemoveBoardMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")
	targetUserID := r.PathValue("userId")

	if boardID == "" || targetUserID == "" {
		http.Error(w, "Board ID and User ID required", http.StatusBadRequest)
		return
	}

	// Check if user can remove members
	role, err := h.store.GetBoardMemberRole(boardID, userID)
	if err != nil || (role != "owner" && role != "admin") {
		http.Error(w, "Not authorized to remove members", http.StatusForbidden)
		return
	}

	// Can't remove the owner
	targetRole, _ := h.store.GetBoardMemberRole(boardID, targetUserID)
	if targetRole == "owner" {
		http.Error(w, "Cannot remove board owner", http.StatusBadRequest)
		return
	}

	err = h.store.RemoveBoardMember(boardID, targetUserID)
	if err != nil {
		http.Error(w, "Failed to remove member", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

// Column handlers

func (h *KanbanHandler) CreateColumn(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.CreateColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Column name is required", http.StatusBadRequest)
		return
	}

	position := -1
	if req.Position != nil {
		position = *req.Position
	}

	column, err := h.store.CreateColumn(boardID, req.Name, position)
	if err != nil {
		http.Error(w, "Failed to create column", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(column)
}

func (h *KanbanHandler) UpdateColumn(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	columnID := r.PathValue("id")

	if columnID == "" {
		http.Error(w, "Column ID required", http.StatusBadRequest)
		return
	}

	column, err := h.store.GetColumn(columnID)
	if err != nil {
		http.Error(w, "Column not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(column.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.UpdateColumnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name != "" {
		err = h.store.UpdateColumn(columnID, req.Name)
		if err != nil {
			http.Error(w, "Failed to update column", http.StatusInternalServerError)
			return
		}
	}

	if req.Position != nil {
		err = h.store.UpdateColumnPosition(columnID, *req.Position)
		if err != nil {
			http.Error(w, "Failed to update column position", http.StatusInternalServerError)
			return
		}
	}

	column, _ = h.store.GetColumn(columnID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(column)
}

func (h *KanbanHandler) DeleteColumn(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	columnID := r.PathValue("id")

	if columnID == "" {
		http.Error(w, "Column ID required", http.StatusBadRequest)
		return
	}

	column, err := h.store.GetColumn(columnID)
	if err != nil {
		http.Error(w, "Column not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(column.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	err = h.store.DeleteColumn(columnID)
	if err != nil {
		http.Error(w, "Failed to delete column", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *KanbanHandler) ReorderColumns(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.ReorderColumnsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.store.ReorderColumns(boardID, req.ColumnIDs)
	if err != nil {
		http.Error(w, "Failed to reorder columns", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "reordered"})
}

// Card handlers

func (h *KanbanHandler) CreateCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		http.Error(w, "Card title is required", http.StatusBadRequest)
		return
	}

	if req.ColumnID == "" {
		http.Error(w, "Column ID is required", http.StatusBadRequest)
		return
	}

	var dueDate *time.Time
	if req.DueDate != "" {
		t, err := time.Parse(time.RFC3339, req.DueDate)
		if err == nil {
			dueDate = &t
		}
	}

	card, err := h.store.CreateCard(req.ColumnID, boardID, req.Title, req.Description, userID, dueDate)
	if err != nil {
		http.Error(w, "Failed to create card", http.StatusInternalServerError)
		return
	}

	// Set assignees if provided
	if len(req.AssigneeIDs) > 0 {
		h.store.SetCardAssignees(card.ID, req.AssigneeIDs)
	}

	// Set labels if provided
	if len(req.LabelIDs) > 0 {
		h.store.SetCardLabels(card.ID, req.LabelIDs)
	}

	// Get full card details
	cardWithDetails, err := h.store.GetCardWithDetails(card.ID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(card)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cardWithDetails)
}

func (h *KanbanHandler) GetCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	cardWithDetails, err := h.store.GetCardWithDetails(cardID)
	if err != nil {
		http.Error(w, "Failed to fetch card details", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cardWithDetails)
}

func (h *KanbanHandler) UpdateCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.UpdateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	title := card.Title
	description := card.Description
	dueDate := card.DueDate

	if req.Title != "" {
		title = req.Title
	}
	if req.Description != "" {
		description = req.Description
	}
	if req.DueDate != "" {
		t, err := time.Parse(time.RFC3339, req.DueDate)
		if err == nil {
			dueDate = &t
		}
	}

	err = h.store.UpdateCard(cardID, title, description, dueDate)
	if err != nil {
		http.Error(w, "Failed to update card", http.StatusInternalServerError)
		return
	}

	// Update assignees if provided
	if req.AssigneeIDs != nil {
		h.store.SetCardAssignees(cardID, req.AssigneeIDs)
	}

	// Update labels if provided
	if req.LabelIDs != nil {
		h.store.SetCardLabels(cardID, req.LabelIDs)
	}

	cardWithDetails, _ := h.store.GetCardWithDetails(cardID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cardWithDetails)
}

func (h *KanbanHandler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	err = h.store.DeleteCard(cardID)
	if err != nil {
		http.Error(w, "Failed to delete card", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *KanbanHandler) MoveCard(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.MoveCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ColumnID == "" {
		http.Error(w, "Column ID is required", http.StatusBadRequest)
		return
	}

	err = h.store.MoveCard(cardID, req.ColumnID, req.Position)
	if err != nil {
		http.Error(w, "Failed to move card", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "moved"})
}

// Label handlers

func (h *KanbanHandler) GetLabels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	labels, err := h.store.GetLabelsForBoard(boardID)
	if err != nil {
		http.Error(w, "Failed to fetch labels", http.StatusInternalServerError)
		return
	}

	if labels == nil {
		labels = []models.KanbanLabel{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(labels)
}

func (h *KanbanHandler) CreateLabel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	boardID := r.PathValue("id")

	if boardID == "" {
		http.Error(w, "Board ID required", http.StatusBadRequest)
		return
	}

	isMember, _ := h.store.IsBoardMember(boardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.CreateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Color == "" {
		http.Error(w, "Name and color are required", http.StatusBadRequest)
		return
	}

	label, err := h.store.CreateLabel(boardID, req.Name, req.Color)
	if err != nil {
		http.Error(w, "Failed to create label", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(label)
}

func (h *KanbanHandler) UpdateLabel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	labelID := r.PathValue("id")

	if labelID == "" {
		http.Error(w, "Label ID required", http.StatusBadRequest)
		return
	}

	label, err := h.store.GetLabel(labelID)
	if err != nil {
		http.Error(w, "Label not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(label.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.UpdateLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := label.Name
	color := label.Color
	if req.Name != "" {
		name = req.Name
	}
	if req.Color != "" {
		color = req.Color
	}

	err = h.store.UpdateLabel(labelID, name, color)
	if err != nil {
		http.Error(w, "Failed to update label", http.StatusInternalServerError)
		return
	}

	label, _ = h.store.GetLabel(labelID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(label)
}

func (h *KanbanHandler) DeleteLabel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	labelID := r.PathValue("id")

	if labelID == "" {
		http.Error(w, "Label ID required", http.StatusBadRequest)
		return
	}

	label, err := h.store.GetLabel(labelID)
	if err != nil {
		http.Error(w, "Label not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(label.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	err = h.store.DeleteLabel(labelID)
	if err != nil {
		http.Error(w, "Failed to delete label", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// Comment handlers

func (h *KanbanHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	comments, err := h.store.GetCommentsForCard(cardID)
	if err != nil {
		http.Error(w, "Failed to fetch comments", http.StatusInternalServerError)
		return
	}

	if comments == nil {
		comments = []models.KanbanComment{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comments)
}

func (h *KanbanHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	cardID := r.PathValue("id")

	if cardID == "" {
		http.Error(w, "Card ID required", http.StatusBadRequest)
		return
	}

	card, err := h.store.GetCard(cardID)
	if err != nil {
		http.Error(w, "Card not found", http.StatusNotFound)
		return
	}

	isMember, _ := h.store.IsBoardMember(card.BoardID, userID)
	if !isMember {
		http.Error(w, "Not a member of this board", http.StatusForbidden)
		return
	}

	var req models.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "Comment content is required", http.StatusBadRequest)
		return
	}

	comment, err := h.store.CreateKanbanComment(cardID, userID, req.Content)
	if err != nil {
		http.Error(w, "Failed to create comment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(comment)
}

func (h *KanbanHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	commentID := r.PathValue("id")

	if commentID == "" {
		http.Error(w, "Comment ID required", http.StatusBadRequest)
		return
	}

	comment, err := h.store.GetKanbanComment(commentID)
	if err != nil {
		http.Error(w, "Comment not found", http.StatusNotFound)
		return
	}

	// Only comment author can delete
	if comment.UserID != userID {
		http.Error(w, "Not authorized to delete this comment", http.StatusForbidden)
		return
	}

	err = h.store.DeleteKanbanComment(commentID)
	if err != nil {
		http.Error(w, "Failed to delete comment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}
