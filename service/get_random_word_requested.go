// service/get_random_word_requested.go

package service

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/machinebox/graphql"
	"github.com/sirupsen/logrus"
	"github.com/tokamak-network/DRB-Node/utils"
)

type PoFClientWrapper struct {
    *utils.PoFClient
}

func NewPoFClientWrapper(client *utils.PoFClient) *PoFClientWrapper {
    return &PoFClientWrapper{PoFClient: client}
}

// GetRandomWordRequested fetches and processes random words requested.
func (l *PoFClientWrapper) GetRandomWordRequested() (*utils.RoundResults, error) {
    config := utils.GetConfig()
    client := graphql.NewClient(config.SubgraphURL)
    ctx := context.Background()

    req := utils.GetRandomWordsRequestedRequest()
    var respData struct {
        RandomWordsRequested []utils.RandomWordRequestedStruct `json:"randomWordsRequesteds"`
    }

    if err := client.Run(ctx, req, &respData); err != nil {
        logrus.Errorf("Error running GraphQL query: %v", err)
        return nil, err
    }

    latestRounds := make(map[string]utils.RandomWordRequestedStruct)
    for _, item := range respData.RandomWordsRequested {
        if existing, ok := latestRounds[item.Round]; ok {
            existingTimestamp, _ := strconv.Atoi(existing.BlockTimestamp)
            currentTimestamp, _ := strconv.Atoi(item.BlockTimestamp)
            if currentTimestamp > existingTimestamp {
                latestRounds[item.Round] = item
            }
        } else {
            latestRounds[item.Round] = item
        }
    }

    var rounds []struct {
        RoundInt int
        Data     utils.RandomWordRequestedStruct
    }

    for round, data := range latestRounds {
        roundInt, err := strconv.Atoi(round)
        if err != nil {
            logrus.Errorf("Error converting round to int: %s, %v", round, err)
            continue
        }
        rounds = append(rounds, struct {
            RoundInt int
            Data     utils.RandomWordRequestedStruct
        }{RoundInt: roundInt, Data: data})
    }

    var filteredRounds []struct {
        RoundInt int
        Data     utils.RandomWordRequestedStruct
    }

    for _, round := range rounds {
        if !round.Data.RoundInfo.IsFulfillExecuted {
            filteredRounds = append(filteredRounds, round)
        }
    }

    sort.Slice(filteredRounds, func(i, j int) bool {
        return filteredRounds[i].RoundInt < filteredRounds[j].RoundInt
    })

    results := &utils.RoundResults{
        RecoverableRounds:           []string{},
        CommittableRounds:           []string{},
        FulfillableRounds:           []string{},
        ReRequestableRounds:         []string{},
        LeadershipDisputeableRounds: []string{},
        CompleteRounds:              []string{},
        RecoveryData:                []utils.RecoveryResult{},
    }

    for _, round := range filteredRounds {
        item := round.Data

        reqOne := utils.GetCommitCsRequest(item.Round, config.WalletAddress)
        var respOneData struct {
            CommitCs []struct {
                BlockTimestamp string `json:"blockTimestamp"`
                CommitVal      string `json:"commitVal"`
            } `json:"commitCs"`
        }

        if err := client.Run(ctx, reqOne, &respOneData); err != nil {
            logrus.Errorf("Error running commitCs query for round %s: %v", item.Round, err)
            continue
        }


        validCommitCount, err := strconv.Atoi(item.RoundInfo.ValidCommitCount)
        if err != nil {
            logrus.Errorf("Error converting ValidCommitCount to int: %v", err)
            continue
        }

        recoveredData, err := GetRecoveredData(item.Round)
        if err != nil {
            logrus.Errorf("Error retrieving recovered data for round %s: %v", item.Round, err)
        }

        var recoverPhaseEndTime time.Time
        var isRecovered bool
        var omega string
        var msgSender string

        for _, data := range recoveredData {
            blockTimestamp, err := strconv.ParseInt(data.BlockTimestamp, 10, 64)
            if err != nil {
                logrus.Errorf("Failed to parse block timestamp for round %s: %v", item.Round, err)
                continue
            }

            isRecovered = data.IsRecovered
            omega = data.Omega
            msgSender = data.MsgSender
            blockTime := time.Unix(blockTimestamp, 0)
            recoverPhaseEndTime = blockTime.Add(DisputeDuration * time.Second)
        }

        fulfillData, err := GetFulfillRandomnessData(item.Round)
        if err != nil {
            logrus.Errorf("Error retrieving fulfill randomness data for round %s: %v", item.Round, err)
        }

        var fulfillSender string
        for _, data := range fulfillData {
            if data.Success {
                fulfillSender = data.MsgSender
                break
            }
        }

        requestBlockTimestampStr := item.BlockTimestamp
        requestBlockTimestampInt, err := strconv.ParseInt(requestBlockTimestampStr, 10, 64)
        if err != nil {
            logrus.Errorf("Error converting request block timestamp to int64: %v", err)
            continue
        }
        requestBlockTimestamp := time.Unix(requestBlockTimestampInt, 0)

        getCommitData, err := GetCommitData(item.Round)
        if err != nil {
            logrus.Errorf("Error retrieving commit data for round %s: %v", item.Round, err)
        }

        var commitSenders []common.Address
        var isCommitSender bool
        var commitTimeStampStr string

        for _, data := range getCommitData {
            commitSender := common.HexToAddress(data.MsgSender)
            commitSenders = append(commitSenders, commitSender)
            commitTimeStampStr = data.BlockTimestamp
        }

        for _, commitSender := range commitSenders {
            if commitSender == common.HexToAddress(config.WalletAddress) {
                isCommitSender = true
                break
            }
        }

        var isMyAddressLeader bool
        var leaderAddress common.Address
        leaderAddress = common.HexToAddress(item.RoundInfo.Leader)
        if leaderAddress == common.HexToAddress(config.WalletAddress) {
            isMyAddressLeader = true
        }

        commitTimeStampInt, err := strconv.ParseInt(commitTimeStampStr, 10, 64)
        if err != nil {
            logrus.Errorf("Error converting commit timestamp to int64: %v", err)
            continue
        }
        commitTimeStampTime := time.Unix(commitTimeStampInt, 0)
        commitPhaseEndTime := commitTimeStampTime.Add(PhaseDuration * time.Second)

        if recoverPhaseEndTime.Before(time.Now()) {
            recoverData := RecoveryResult{
                Round:               item.Round,
                RecoveredBlockTime:  recoverPhaseEndTime.Format(time.RFC3339),
                RequestBlockTime:    requestBlockTimestamp.Format(time.RFC3339),
                CommitPhaseEndTime:  commitPhaseEndTime.Format(time.RFC3339),
                RecoverPhaseEndTime: recoverPhaseEndTime.Format(time.RFC3339),
                IsRecovered:         isRecovered,
                IsFulfillExecuted:   item.RoundInfo.IsFulfillExecuted,
            }
            results.CompleteRounds = append(results.CompleteRounds, item.Round)
            results.RecoveryData = append(results.RecoveryData, recoverData)
        } else {
            if validCommitCount > 0 && !isCommitSender {
                results.CommittableRounds = append(results.CommittableRounds, item.Round)
            } else if isMyAddressLeader {
                results.LeaderRounds = append(results.LeaderRounds, item.Round)
            } else if commitTimeStampTime.After(time.Now()) {
                results.ReRequestableRounds = append(results.ReRequestableRounds, item.Round)
            } else {
                results.FulfillableRounds = append(results.FulfillableRounds, item.Round)
            }
        }
    }

    return results, nil
}
