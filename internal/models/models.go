package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleStaff    Role = "staff"
	RoleCustomer Role = "customer"
	RoleSystem   Role = "system"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Customer struct {
	ID         uuid.UUID
	UserID     *uuid.UUID
	FullName   string
	Phone      *string
	DocumentID *string
	Address    *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Branch struct {
	ID        uuid.UUID
	Code      string
	Name      string
	Address   string
	Latitude  *float64
	Longitude *float64
	Phone     *string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Collection struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Season      *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Product struct {
	ID              uuid.UUID
	CollectionID    *uuid.UUID
	SKU             string
	Name            string
	Description     *string
	Category        string
	BasePrice       float64
	Currency        string
	ImageURL        *string
	ImageDocumentID *uuid.UUID
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ProductVariant struct {
	ID            uuid.UUID
	ProductID     uuid.UUID
	SKU           string
	Size          string
	Color         string
	PriceOverride *float64
	IsActive      bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Inventory struct {
	ID           uuid.UUID
	VariantID    uuid.UUID
	BranchID     uuid.UUID
	Quantity     int
	ReorderLevel int
	UpdatedAt    time.Time
}

type SaleStatus string

const (
	SaleStatusPending   SaleStatus = "pending"
	SaleStatusConfirmed SaleStatus = "confirmed"
	SaleStatusCancelled SaleStatus = "cancelled"
)

type Sale struct {
	ID          uuid.UUID
	CustomerID  *uuid.UUID
	BranchID    uuid.UUID
	CashierID   *uuid.UUID
	Status      SaleStatus
	Subtotal    float64
	Tax         float64
	Total       float64
	Currency    string
	ConfirmedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type SaleItem struct {
	ID        uuid.UUID
	SaleID    uuid.UUID
	VariantID uuid.UUID
	Quantity  int
	UnitPrice float64
	LineTotal float64
	CreatedAt time.Time
}

type OrderStatus string

const (
	OrderStatusPlaced    OrderStatus = "placed"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID        uuid.UUID
	SaleID    uuid.UUID
	Code      string
	Status    OrderStatus
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PushPlatform string

const (
	PushPlatformIOS     PushPlatform = "ios"
	PushPlatformAndroid PushPlatform = "android"
	PushPlatformWeb     PushPlatform = "web"
)

type PushToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Token      string
	Platform   PushPlatform
	DeviceID   *string
	IsActive   bool
	LastSeenAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WebhookEvent struct {
	ID            uuid.UUID
	EventType     string
	AggregateID   uuid.UUID
	Payload       []byte
	TargetURL     string
	Status        string
	Attempts      int
	LastError     *string
	NextAttemptAt time.Time
	DeliveredAt   *time.Time
	CreatedAt     time.Time
}
