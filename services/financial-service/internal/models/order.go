package models

import "time"

type Order struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	Asset     string    `db:"asset"`
	Amount    float64   `db:"amount"`
	Status    int32     `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Transaction struct {
	ID          string    `db:"id"` // VARCHAR PK like TR-xxxxx
	UserID      uint64    `db:"user_id"`
	Asset       string    `db:"asset"`
	Amount      float64   `db:"amount"`
	Action      string    `db:"action"` // deposit, withdraw
	Status      int32     `db:"status"`
	Token       *int64    `db:"token"`
	RefID       *int64    `db:"ref_id"`
	PayableType *string   `db:"payable_type"`
	PayableID   *uint64   `db:"payable_id"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type Payment struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	RefID     int64     `db:"ref_id"`
	CardPan   string    `db:"card_pan"`
	Gateway   string    `db:"gateway"`
	Amount    float64   `db:"amount"`
	Product   string    `db:"product"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Option struct {
	ID        uint64    `db:"id"`
	Code      string    `db:"code"`
	Asset     string    `db:"asset"`
	Amount    float64   `db:"amount"`
	Note      *string   `db:"note"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Variable struct {
	ID    uint64  `db:"id"`
	Key   string  `db:"key"`
	Value float64 `db:"value"`
}

type FirstOrder struct {
	ID        uint64    `db:"id"`
	UserID    uint64    `db:"user_id"`
	Type      string    `db:"type"`
	Amount    float64   `db:"amount"`
	Date      string    `db:"date"` // Jalali date format Y/m/d
	Bonus     float64   `db:"bonus"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type User struct {
	ID        uint64     `db:"id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	Phone     string     `db:"phone"`
	Birthdate *time.Time `db:"birthdate"`
}
