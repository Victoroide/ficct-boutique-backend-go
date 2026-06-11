package graph

import (
	"github.com/ficct-boutique/backend-go/internal/repository"
	"github.com/ficct-boutique/backend-go/internal/service"
)

// Resolver is the root GraphQL resolver. It is the dependency-injection
// container wiring every service and repository used to satisfy queries and
// mutations; the generated schema binds its accessor methods (see types.go) to
// the corresponding GraphQL fields.
type Resolver struct {
	AuthSvc    *service.AuthService
	CatalogSvc *service.CatalogService
	SalesSvc   *service.SalesService
	ReportsSvc *service.ReportsService
	PushSender *service.PushSender

	UserRepo      *repository.UserRepo
	CatalogRepo   *repository.CatalogRepo
	BranchRepo    *repository.BranchRepo
	InvRepo       *repository.InventoryRepo
	SalesRepo     *repository.SalesRepo
	OrderRepo     *repository.OrderRepo
	PushTokenRepo *repository.PushTokenRepo
}
