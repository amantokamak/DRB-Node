package service

import (
	"github.com/tokamak-network/DRB-Node/utils"
)

// ServiceClient provides methods for interacting with the contract.
type ServiceClient struct {
	PoFClient *utils.PoFClient
}

// NewServiceClient creates a new ServiceClient.
func NewServiceClient(pofClient *utils.PoFClient) *ServiceClient {
	return &ServiceClient{PoFClient: pofClient}
}
