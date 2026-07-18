package repository

import (
	"context"
	"database/sql"
	"fmt"

	"metarang/support-service/internal/models"
)

type TicketRepository interface {
	Create(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error)
	GetByID(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error)
	GetByUserID(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error)
	Update(ctx context.Context, ticket *models.Ticket) error
	UpdateStatus(ctx context.Context, ticketID uint64, status int32) error
	GetResponsesByTicketID(ctx context.Context, ticketID uint64) ([]models.TicketResponse, error)
	CreateResponse(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error)
	CheckUserOwnership(ctx context.Context, ticketID, userID uint64) (bool, error)
	GetTicketSenderReceiver(ctx context.Context, ticketID uint64) (senderID, receiverID uint64, err error)
}

type ticketRepository struct {
	db *sql.DB
}

func NewTicketRepository(db *sql.DB) TicketRepository {
	return &ticketRepository{db: db}
}

func (r *ticketRepository) Create(ctx context.Context, ticket *models.Ticket) (*models.Ticket, error) {
	query := `
		INSERT INTO tickets (title, content, attachment, status, department, importance, code, user_id, reciever_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		ticket.Title,
		ticket.Content,
		ticket.Attachment,
		ticket.Status,
		ticket.Department,
		ticket.Importance,
		ticket.Code,
		ticket.UserID,
		ticket.ReceiverID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	ticket.ID = uint64(id)
	return ticket, nil
}

func (r *ticketRepository) GetByID(ctx context.Context, ticketID uint64) (*models.TicketWithRelations, error) {
	query := `
		SELECT 
			t.id, t.title, t.content, t.attachment, t.status, t.department, t.importance, t.code,
			t.user_id, t.reciever_id, t.created_at, t.updated_at,
			sender.name as sender_name, sender.code as sender_code,
			receiver.name as receiver_name, receiver.code as receiver_code,
			sender_photo.url as sender_photo_url,
			receiver_photo.url as receiver_photo_url
		FROM tickets t
		INNER JOIN users sender ON t.user_id = sender.id
		LEFT JOIN users receiver ON t.reciever_id = receiver.id
		LEFT JOIN (
			SELECT imageable_id AS user_id, url
			FROM images
			WHERE imageable_type = 'App\\Models\\User'
			  AND id IN (
				SELECT MAX(id) FROM images
				WHERE imageable_type = 'App\\Models\\User'
				GROUP BY imageable_id
			  )
		) sender_photo ON sender.id = sender_photo.user_id
		LEFT JOIN (
			SELECT imageable_id AS user_id, url
			FROM images
			WHERE imageable_type = 'App\\Models\\User'
			  AND id IN (
				SELECT MAX(id) FROM images
				WHERE imageable_type = 'App\\Models\\User'
				GROUP BY imageable_id
			  )
		) receiver_photo ON receiver.id = receiver_photo.user_id
		WHERE t.id = ?
	`

	var ticket models.TicketWithRelations
	var receiverName, receiverCode sql.NullString
	var receiverID sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, ticketID).Scan(
		&ticket.ID, &ticket.Title, &ticket.Content, &ticket.Attachment,
		&ticket.Status, &ticket.Department, &ticket.Importance, &ticket.Code,
		&ticket.UserID, &receiverID, &ticket.CreatedAt, &ticket.UpdatedAt,
		&ticket.SenderName, &ticket.SenderCode,
		&receiverName, &receiverCode,
		&ticket.SenderProfilePhoto, &ticket.ReceiverProfilePhoto,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ticket: %w", err)
	}

	if receiverID.Valid {
		rid := uint64(receiverID.Int64)
		ticket.ReceiverID = &rid
	}
	if receiverName.Valid {
		ticket.ReceiverName = &receiverName.String
	}
	if receiverCode.Valid {
		ticket.ReceiverCode = &receiverCode.String
	}

	// Load responses
	responses, err := r.GetResponsesByTicketID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	ticket.Responses = responses

	return &ticket, nil
}

func (r *ticketRepository) GetByUserID(ctx context.Context, userID uint64, page, perPage int32, received bool) ([]*models.TicketWithRelations, int, error) {
	// Count total tickets
	var countQuery string
	if received {
		countQuery = `SELECT COUNT(*) FROM tickets WHERE reciever_id = ?`
	} else {
		countQuery = `SELECT COUNT(*) FROM tickets WHERE user_id = ?`
	}

	var total int
	var err error
	if received {
		err = r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	} else {
		err = r.db.QueryRowContext(ctx, countQuery, userID).Scan(&total)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count tickets: %w", err)
	}

	// Get tickets with pagination
	offset := (page - 1) * perPage
	query := `
		SELECT 
			t.id, t.title, t.content, t.attachment, t.status, t.department, t.importance, t.code,
			t.user_id, t.reciever_id, t.created_at, t.updated_at,
			sender.name as sender_name, sender.code as sender_code,
			receiver.name as receiver_name, receiver.code as receiver_code,
			sender_photo.url as sender_photo_url,
			receiver_photo.url as receiver_photo_url
		FROM tickets t
		INNER JOIN users sender ON t.user_id = sender.id
		LEFT JOIN users receiver ON t.reciever_id = receiver.id
		LEFT JOIN (
			SELECT imageable_id AS user_id, url
			FROM images
			WHERE imageable_type = 'App\\Models\\User'
			  AND id IN (
				SELECT MAX(id) FROM images
				WHERE imageable_type = 'App\\Models\\User'
				GROUP BY imageable_id
			  )
		) sender_photo ON sender.id = sender_photo.user_id
		LEFT JOIN (
			SELECT imageable_id AS user_id, url
			FROM images
			WHERE imageable_type = 'App\\Models\\User'
			  AND id IN (
				SELECT MAX(id) FROM images
				WHERE imageable_type = 'App\\Models\\User'
				GROUP BY imageable_id
			  )
		) receiver_photo ON receiver.id = receiver_photo.user_id
	`

	if received {
		query += " WHERE t.reciever_id = ?"
	} else {
		query += " WHERE t.user_id = ?"
	}

	query += " ORDER BY t.updated_at DESC LIMIT ? OFFSET ?"

	rows, err := r.db.QueryContext(ctx, query, userID, perPage, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get tickets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tickets []*models.TicketWithRelations
	for rows.Next() {
		var ticket models.TicketWithRelations
		var receiverName, receiverCode sql.NullString
		var receiverID sql.NullInt64

		err := rows.Scan(
			&ticket.ID, &ticket.Title, &ticket.Content, &ticket.Attachment,
			&ticket.Status, &ticket.Department, &ticket.Importance, &ticket.Code,
			&ticket.UserID, &receiverID, &ticket.CreatedAt, &ticket.UpdatedAt,
			&ticket.SenderName, &ticket.SenderCode,
			&receiverName, &receiverCode,
			&ticket.SenderProfilePhoto, &ticket.ReceiverProfilePhoto,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan ticket: %w", err)
		}

		if receiverID.Valid {
			rid := uint64(receiverID.Int64)
			ticket.ReceiverID = &rid
		}
		if receiverName.Valid {
			ticket.ReceiverName = &receiverName.String
		}
		if receiverCode.Valid {
			ticket.ReceiverCode = &receiverCode.String
		}

		tickets = append(tickets, &ticket)
	}

	return tickets, total, nil
}

func (r *ticketRepository) Update(ctx context.Context, ticket *models.Ticket) error {
	query := `
		UPDATE tickets 
		SET title = ?, content = ?, attachment = ?, status = ?, updated_at = NOW()
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query,
		ticket.Title,
		ticket.Content,
		ticket.Attachment,
		ticket.Status,
		ticket.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update ticket: %w", err)
	}

	return nil
}

func (r *ticketRepository) UpdateStatus(ctx context.Context, ticketID uint64, status int32) error {
	query := `UPDATE tickets SET status = ?, updated_at = NOW() WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, status, ticketID)
	if err != nil {
		return fmt.Errorf("failed to update ticket status: %w", err)
	}

	return nil
}

func (r *ticketRepository) GetResponsesByTicketID(ctx context.Context, ticketID uint64) ([]models.TicketResponse, error) {
	query := `
		SELECT id, ticket_id, response, attachment, responser_name, responser_id, created_at, updated_at
		FROM ticket_responses
		WHERE ticket_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to get responses: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var responses []models.TicketResponse
	for rows.Next() {
		var response models.TicketResponse
		err := rows.Scan(
			&response.ID, &response.TicketID, &response.Response,
			&response.Attachment, &response.ResponserName, &response.ResponserID,
			&response.CreatedAt, &response.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan response: %w", err)
		}
		responses = append(responses, response)
	}

	return responses, nil
}

func (r *ticketRepository) CreateResponse(ctx context.Context, response *models.TicketResponse) (*models.TicketResponse, error) {
	query := `
		INSERT INTO ticket_responses (ticket_id, response, attachment, responser_name, responser_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
	`

	result, err := r.db.ExecContext(ctx, query,
		response.TicketID,
		response.Response,
		response.Attachment,
		response.ResponserName,
		response.ResponserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create response: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	response.ID = uint64(id)
	return response, nil
}

func (r *ticketRepository) CheckUserOwnership(ctx context.Context, ticketID, userID uint64) (bool, error) {
	query := `SELECT COUNT(*) FROM tickets WHERE id = ? AND (user_id = ? OR reciever_id = ?)`

	var count int
	err := r.db.QueryRowContext(ctx, query, ticketID, userID, userID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check ownership: %w", err)
	}

	return count > 0, nil
}

func (r *ticketRepository) GetTicketSenderReceiver(ctx context.Context, ticketID uint64) (senderID, receiverID uint64, err error) {
	query := `SELECT user_id, reciever_id FROM tickets WHERE id = ?`

	var recID sql.NullInt64
	err = r.db.QueryRowContext(ctx, query, ticketID).Scan(&senderID, &recID)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get sender/receiver: %w", err)
	}

	if recID.Valid {
		receiverID = uint64(recID.Int64)
	}

	return senderID, receiverID, nil
}
