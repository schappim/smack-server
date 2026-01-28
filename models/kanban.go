package models

import "time"

// Board represents a kanban board
type Board struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BoardWithDetails includes additional computed fields
type BoardWithDetails struct {
	Board
	ColumnCount int `json:"column_count"`
	CardCount   int `json:"card_count"`
	MemberCount int `json:"member_count"`
}

// BoardMember represents a user's membership in a board
type BoardMember struct {
	BoardID  string       `json:"board_id"`
	UserID   string       `json:"user_id"`
	Role     string       `json:"role"` // "owner", "admin", "member"
	JoinedAt time.Time    `json:"joined_at"`
	User     UserResponse `json:"user,omitempty"`
}

// KanbanColumn represents a column in a kanban board
type KanbanColumn struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// KanbanColumnWithCards includes the cards in the column
type KanbanColumnWithCards struct {
	KanbanColumn
	Cards []CardWithDetails `json:"cards"`
}

// KanbanLabel represents a label that can be applied to cards
type KanbanLabel struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"` // hex color like "#FF5733"
	CreatedAt time.Time `json:"created_at"`
}

// KanbanCard represents a card in a kanban column
type KanbanCard struct {
	ID          string     `json:"id"`
	ColumnID    string     `json:"column_id"`
	BoardID     string     `json:"board_id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Position    int        `json:"position"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedBy   string     `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// CardWithDetails includes assignees, labels, and comment count
type CardWithDetails struct {
	KanbanCard
	Assignees    []UserResponse `json:"assignees"`
	Labels       []KanbanLabel  `json:"labels"`
	CommentCount int            `json:"comment_count"`
	Creator      UserResponse   `json:"creator,omitempty"`
}

// KanbanComment represents a comment on a card
type KanbanComment struct {
	ID        string       `json:"id"`
	CardID    string       `json:"card_id"`
	UserID    string       `json:"user_id"`
	Content   string       `json:"content"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	User      UserResponse `json:"user,omitempty"`
}

// Request types

type CreateBoardRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

type UpdateBoardRequest struct {
	Name        string  `json:"name,omitempty"`
	Description string  `json:"description,omitempty"`
	Icon        *string `json:"icon,omitempty"`
}

type CreateColumnRequest struct {
	Name     string `json:"name"`
	Position *int   `json:"position,omitempty"`
}

type UpdateColumnRequest struct {
	Name     string `json:"name,omitempty"`
	Position *int   `json:"position,omitempty"`
}

type ReorderColumnsRequest struct {
	ColumnIDs []string `json:"column_ids"`
}

type CreateCardRequest struct {
	ColumnID    string   `json:"column_id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	DueDate     string   `json:"due_date,omitempty"` // ISO8601
	AssigneeIDs []string `json:"assignee_ids,omitempty"`
	LabelIDs    []string `json:"label_ids,omitempty"`
}

type UpdateCardRequest struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	DueDate     string   `json:"due_date,omitempty"`
	AssigneeIDs []string `json:"assignee_ids,omitempty"`
	LabelIDs    []string `json:"label_ids,omitempty"`
}

type MoveCardRequest struct {
	ColumnID string `json:"column_id"`
	Position int    `json:"position"`
}

type CreateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

type UpdateLabelRequest struct {
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
}

type CreateCommentRequest struct {
	Content string `json:"content"`
}

type AddBoardMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role,omitempty"` // defaults to "member"
}
