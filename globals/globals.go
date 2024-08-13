package globals

import (
	"github.com/tokamak-network/DRB-Node/service"
	"github.com/tokamak-network/DRB-Node/utils"
)

// GlobalServiceClient will be used to access the global ServiceClient
var GlobalServiceClient *service.ServiceClient

// Init initializes the global ServiceClient instance
func Init(pofClient *utils.PoFClient) {
	GlobalServiceClient = service.NewServiceClient(pofClient)
}
