package relay

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"sync"

	relaychain "github.com/keep-network/keep-core/pkg/beacon/relay/chain"
	"github.com/keep-network/keep-core/pkg/beacon/relay/config"
	"github.com/keep-network/keep-core/pkg/beacon/relay/dkg"
	"github.com/keep-network/keep-core/pkg/beacon/relay/groupselection"
	"github.com/keep-network/keep-core/pkg/beacon/relay/registry"
	"github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-core/pkg/net"
)

// Node represents the current state of a relay node.
type Node struct {
	mutex sync.Mutex

	// Staker is an on-chain identity that this node is using to prove its
	// stake in the system.
	Staker chain.Staker

	// External interactors.
	netProvider  net.Provider
	blockCounter chain.BlockCounter
	chainConfig  *config.Chain

	groupRegistry *registry.Groups
}

// IsInGroup checks if this node is a member of the group which was selected to
// join a group which undergoes the process of generating a threshold relay entry.
func (n *Node) IsInGroup(groupPublicKey []byte) bool {
	return len(n.groupRegistry.GetGroup(groupPublicKey)) > 0
}

// JoinGroupIfEligible takes a threshold relay entry value and undergoes the
// process of joining a group if this node's virtual stakers prove eligible for
// the group generated by that entry. This is an interactive on-chain process,
// and JoinGroupIfEligible can block for an extended period of time while it
// completes the on-chain operation.
//
// Indirectly, the completion of the process is signaled by the formation of an
// on-chain group containing at least one of this node's virtual stakers.
func (n *Node) JoinGroupIfEligible(
	relayChain relaychain.Interface,
	signing chain.Signing,
	groupSelectionResult *groupselection.Result,
	newEntry *big.Int,
) {
	dkgStartBlockHeight := groupSelectionResult.GroupSelectionEndBlock

	indexes := make([]int, 0)
	for index, selectedStaker := range groupSelectionResult.SelectedStakers {
		// See if we are amongst those chosen
		if bytes.Compare(selectedStaker, n.Staker.ID()) == 0 {
			indexes = append(indexes, index)
		}
	}

	if len(indexes) > 0 {
		// create temporary broadcast channel for DKG using the group selection
		// seed
		broadcastChannel, err := n.netProvider.ChannelFor(newEntry.Text(16))
		if err != nil {
			logger.Errorf("failed to get broadcast channel: [%v]", err)
			return
		}

		err = broadcastChannel.SetFilter(
			createGroupMemberFilter(groupSelectionResult.SelectedStakers, signing),
		)
		if err != nil {
			logger.Errorf(
				"could not add filter for channel [%v]: [%v]",
				broadcastChannel.Name(),
				err,
			)
		}

		for _, index := range indexes {
			// capture player index for goroutine
			playerIndex := index

			go func() {
				signer, err := dkg.ExecuteDKG(
					newEntry,
					playerIndex,
					n.chainConfig.GroupSize,
					n.chainConfig.DishonestThreshold(),
					dkgStartBlockHeight,
					n.blockCounter,
					relayChain,
					signing,
					broadcastChannel,
				)
				if err != nil {
					logger.Errorf("failed to execute dkg: [%v]", err)
					return
				}

				// final broadcast channel name for group is the compressed
				// public key of the group
				channelName := hex.EncodeToString(
					signer.GroupPublicKeyBytesCompressed(),
				)

				err = n.groupRegistry.RegisterGroup(signer, channelName)
				if err != nil {
					logger.Errorf("failed to register a group: [%v]", err)
				}
			}()
		}
	}

	return
}

func createGroupMemberFilter(
	members []relaychain.StakerAddress,
	signing chain.Signing,
) net.BroadcastChannelFilter {
	authorizations := make(map[string]bool, len(members))
	for _, address := range members {
		authorizations[hex.EncodeToString(address)] = true
	}

	return func(authorPublicKey *ecdsa.PublicKey) bool {
		authorAddress := hex.EncodeToString(
			signing.PublicKeyToAddress(*authorPublicKey),
		)
		_, isAuthorized := authorizations[authorAddress]

		if !isAuthorized {
			logger.Debugf(
				"rejecting message from [%v]; author is not a member of the group",
				authorAddress,
			)
		}

		return isAuthorized
	}
}
