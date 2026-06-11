package models

import (
	"time"

	"github.com/google/uuid"
)

// Role is a user's authorization role.
type Role string

const (
	RoleAdmin    Role = "admin"
	RoleStaff    Role = "staff"
	RoleCustomer Role = "customer"
	RoleSystem   Role = "system"
)

// User is an application user account.
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

// Customer is a buyer profile, optionally linked to a user account.
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

// Branch is a physical store location.
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

// Collection is a named grouping of products (e.g. a seasonal line).
type Collection struct {
	ID          uuid.UUID
	Name        string
	Description *string
	Season      *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Product is a catalog item, priced with a base price and sold via its variants.
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

// ProductVariant is a sellable variation of a product (size/color), optionally
// overriding the product's base price.
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

// Inventory is the stock level of a variant at a specific branch.
type Inventory struct {
	ID           uuid.UUID
	VariantID    uuid.UUID
	BranchID     uuid.UUID
	Quantity     int
	ReorderLevel int
	UpdatedAt    time.Time
}

// SaleStatus is the lifecycle state of a sale.
type SaleStatus string

const (
	SaleStatusPending   SaleStatus = "pending"
	SaleStatusConfirmed SaleStatus = "confirmed"
	SaleStatusCancelled SaleStatus = "cancelled"
)

// Sale is a point-of-sale transaction with its computed totals.
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

// SaleItem is one priced line within a sale.
type SaleItem struct {
	ID        uuid.UUID
	SaleID    uuid.UUID
	VariantID uuid.UUID
	Quantity  int
	UnitPrice float64
	LineTotal float64
	CreatedAt time.Time
}

// OrderStatus is the fulfillment state of an order.
type OrderStatus string

const (
	OrderStatusPlaced    OrderStatus = "placed"
	OrderStatusPreparing OrderStatus = "preparing"
	OrderStatusReady     OrderStatus = "ready"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// Order is a fulfillment record created from a confirmed sale.
type Order struct {
	ID        uuid.UUID
	SaleID    uuid.UUID
	Code      string
	Status    OrderStatus
	Notes     *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PushPlatform identifies the device platform a push token belongs to.
type PushPlatform string

const (
	PushPlatformIOS     PushPlatform = "ios"
	PushPlatformAndroid PushPlatform = "android"
	PushPlatformWeb     PushPlatform = "web"
)

// PushToken is a registered device token for delivering push notifications.
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

// WebhookEvent is a transactional-outbox row tracking the delivery of a webhook.
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
