package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"time"

	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tokamak-network/DRB-Node/globals"
	"github.com/tokamak-network/DRB-Node/utils"
)

var roundStatus sync.Map

func ProcessRoundResults(ctx context.Context) error  {
	serviceClient := globals.GlobalServiceClient
	
	config := utils.GetConfig()
	isOperator, err := IsOperator(config.WalletAddress)
	if err != nil {
		log.Printf("Error fetching isOperator results: %v", err)
		return err
	}

	if !isOperator {
		ctx := context.Background()
		serviceClient.OperatorDeposit(ctx)
	}

	results, err := l.GetRandomWordRequested()
	if err != nil {
		log.Printf("Error fetching round results: %v", err)
		return err
	}

	if len(results.RecoverableRounds) > 0 {
		fmt.Println("Processing Recoverable Rounds...")
		processedRounds := make(map[string]bool)

		for _, roundStr := range results.RecoverableRounds {
			if processedRounds[roundStr] {
				continue
			}

			for _, recoveryData := range results.RecoveryData {
				isMyAddressLeader, _, _ := FindOffChainLeaderAtRound(roundStr, recoveryData.OmegaRecov)
				if isMyAddressLeader {
					round := new(big.Int)
					round, ok := round.SetString(roundStr, 10)
					if !ok {
						log.Printf("Failed to convert round string to big.Int: %s", roundStr)
						continue
					}

					ctx := context.Background()
					l.Recover(ctx, round, recoveryData.Y)
					fmt.Printf("Processing recoverable round: %s\n", roundStr)
					processedRounds[roundStr] = true
					time.Sleep(3 * time.Second)
					break
				}
			}

			if !processedRounds[roundStr] {
				fmt.Printf("Not recoverable round: %s\n", roundStr)
			}
		}
	}

	if len(results.CommittableRounds) > 0 {
		fmt.Println("Processing Committable Rounds...")
		processedRounds := make(map[string]bool)

		for _, roundStr := range results.CommittableRounds {
			if processedRounds[roundStr] {
				continue
			}

			round := new(big.Int)
			round, ok := round.SetString(roundStr, 10)
			if !ok {
				log.Printf("Failed to convert round string to big.Int: %s", roundStr)
				continue
			}

			ctx := context.Background()
			l.Commit(ctx, round)
			fmt.Printf("Processing committable round: %s\n", roundStr)
			processedRounds[roundStr] = true
		}
	}

	if len(results.FulfillableRounds) > 0 {
		fmt.Println("Processing Fulfillable Rounds...")
		for _, roundStr := range results.FulfillableRounds {
			round := new(big.Int)
			round, ok := round.SetString(roundStr, 10)
			if !ok {
				log.Printf("Failed to convert round string to big.Int: %s", roundStr)
				continue
			}

			ctx := context.Background()
			l.FulfillRandomness(ctx, round)
		}
	}

	if len(results.ReRequestableRounds) > 0 {
		fmt.Println("Processing ReRequestable Rounds...")
		for _, roundStr := range results.ReRequestableRounds {
			round := new(big.Int)
			round, ok := round.SetString(roundStr, 10)
			if !ok {
				log.Printf("Failed to convert round string to big.Int: %s", roundStr)
				continue
			}

			ctx := context.Background()
			l.ReRequestRandomWordAtRound(ctx, round)
			fmt.Printf("Processing re-requestable round: %s\n", roundStr)
		}
	}

	if len(results.RecoverDisputeableRounds) > 0 {
		fmt.Println("Processing Recover Disputeable Rounds...")
		for _, roundStr := range results.RecoverDisputeableRounds {
			recoveredData, err := GetRecoveredData(roundStr)
			if err != nil {
				log.Printf("Error retrieving recovered data for round %s: %v", roundStr, err)
				continue
			}

			round := new(big.Int)
			round, ok := round.SetString(roundStr, 10)
			if !ok {
				log.Printf("Failed to convert round string to big.Int: %s", roundStr)
				continue
			}

			disputeInitiated := false

			for _, data := range recoveredData {
				msgSender := common.HexToAddress(data.MsgSender)
				omega := new(big.Int)
				omega, ok := omega.SetString(data.Omega[2:], 16)
				if !ok {
					log.Printf("Failed to parse omega for round %s: %s", roundStr, data.Omega)
					continue
				}

				fmt.Printf("Recovered Data - MsgSender: %s, Omega: %s\n", msgSender.Hex(), omega.String())

				for _, recoveryData := range results.RecoveryData {
					if recoveryData.OmegaRecov.Cmp(omega) != 0 && !disputeInitiated {
						ctx := context.Background()
						l.DisputeRecover(ctx, round, recoveryData.V, recoveryData.X, recoveryData.Y)
						disputeInitiated = true
					}
				}

				if disputeInitiated {
					fmt.Printf("Processing disputeable round: %s\n", roundStr)
					break
				}
			}

			if !disputeInitiated {
				fmt.Printf("No disputes initiated for round: %s\n", roundStr)
			}
		}
	}

	if len(results.LeadershipDisputeableRounds) > 0 {
		fmt.Println("Processing Leadership Disputeable Rounds...")
		for i, roundStr := range results.LeadershipDisputeableRounds {
			recoveredData, err := GetRecoveredData(roundStr)
			if err != nil {
				log.Printf("Error retrieving recovered data for round %s: %v", roundStr, err)
				continue
			}

			round := new(big.Int)
			round, ok := round.SetString(roundStr, 10)
			if !ok {
				log.Printf("Failed to convert round string to big.Int: %s", roundStr)
				continue
			}

			var msgSender common.Address

			for _, data := range recoveredData {
				msgSender = common.HexToAddress(data.MsgSender)
				fmt.Printf("Recovered Data - MsgSender: %s\n", msgSender.Hex())
			}

			if i < len(results.RecoveryData) {
				isMyAddressLeader, leaderAddress, _ := FindOffChainLeaderAtRound(roundStr, results.RecoveryData[i].OmegaRecov)

				if msgSender != leaderAddress {
					ctx := context.Background()
					if isMyAddressLeader {
						l.DisputeLeadershipAtRound(ctx, round)
						fmt.Printf("MsgSender %s is not the leader for round %s\n", msgSender.Hex(), roundStr)
					}
				}

				fmt.Printf("Processing disputeable round: %s\n", roundStr)
			} else {
				log.Printf("No recovery data available for round: %s", roundStr)
			}
		}
	}

	return nil
}

