package gjkr

import (
	crand "crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	bn256 "github.com/ethereum/go-ethereum/crypto/bn256/cloudflare"
	"github.com/keep-network/keep-core/pkg/net/ephemeral"
)

func TestCombineReceivedShares(t *testing.T) {
	threshold := 3
	groupSize := 7

	selfShareS := big.NewInt(9)

	receivedShareS := make(map[MemberID]*big.Int)
	// Simulate shares received from peer members.
	// Peer members IDs are in [100, 101, 102, 103, 104, 105] to differ them from
	// slice indices.
	for i := 0; i <= 5; i++ {
		receivedShareS[MemberID(100+i)] = big.NewInt(int64(10 + i))
	}

	// 9 + 10 + 11 + 12 + 13 + 14 + 15 = 84
	expectedShareS := big.NewInt(84)

	members, err := initializeQualifiedMembersGroup(threshold, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	member := members[0]

	// Replace initialized values with values declared at the begining.
	member.selfSecretShareS = selfShareS
	member.receivedValidSharesS = receivedShareS

	member.CombineMemberShares()

	if member.groupPrivateKeyShare.Cmp(expectedShareS) != 0 {
		t.Errorf("incorrect combined shares S value\nexpected: %v\nactual:   %v\n",
			expectedShareS,
			member.groupPrivateKeyShare,
		)
	}
}

func TestCalculatePublicCoefficients(t *testing.T) {
	secretCoefficients := []*big.Int{
		big.NewInt(3),
		big.NewInt(5),
		big.NewInt(2),
	}
	expectedPublicCoefficients := make([]*bn256.G1, len(secretCoefficients))
	for i, secretCoefficient := range secretCoefficients {
		expectedPublicCoefficients[i] = new(bn256.G1).ScalarBaseMult(
			secretCoefficient,
		)
	}

	member := (&LocalMember{
		memberCore: &memberCore{
			protocolParameters: newProtocolParameters(big.NewInt(8328121)),
		},
	}).InitializeEphemeralKeysGeneration().
		InitializeSymmetricKeyGeneration().
		InitializeCommitting().
		InitializeCommitmentsVerification().
		InitializeSharesJustification().
		InitializeQualified().
		InitializeSharing()

	member.secretCoefficients = secretCoefficients

	message := member.CalculatePublicKeySharePoints()

	if !reflect.DeepEqual(member.publicKeySharePoints, expectedPublicCoefficients) {
		t.Errorf("incorrect member's public shares\nexpected: %v\nactual:   %v\n",
			expectedPublicCoefficients,
			member.publicKeySharePoints,
		)
	}

	if !reflect.DeepEqual(message.publicKeySharePoints, expectedPublicCoefficients) {
		t.Errorf("incorrect public shares in message\nexpected: %v\nactual:   %v\n",
			expectedPublicCoefficients,
			message.publicKeySharePoints,
		)
	}
}

func TestCalculateAndVerifyPublicKeySharePoints(t *testing.T) {
	threshold := 3
	groupSize := 5

	sharingMembers, err := initializeSharingMembersGroup(threshold, groupSize)
	if err != nil {
		t.Fatalf("group initialization failed [%s]", err)
	}

	sharingMember := sharingMembers[0]

	var tests = map[string]struct {
		modifyPublicKeySharePointsMessages func(messages []*MemberPublicKeySharePointsMessage)
		expectedError                      error
		expectedAccusedIDs                 []MemberID
	}{
		"positive validation - no accusations": {
			expectedError: nil,
		},
		"negative validation - changed public key share - one accused member": {
			modifyPublicKeySharePointsMessages: func(messages []*MemberPublicKeySharePointsMessage) {
				messages[1].publicKeySharePoints[1] = new(bn256.G2).ScalarMult(
					messages[1].publicKeySharePoints[1],
					big.NewInt(2),
				)

			},
			expectedError:      nil,
			expectedAccusedIDs: []MemberID{3},
		},
		"negative validation - changed public key share - two accused members": {
			modifyPublicKeySharePointsMessages: func(messages []*MemberPublicKeySharePointsMessage) {
				messages[0].publicKeySharePoints[1] = new(bn256.G2).ScalarMult(
					messages[0].publicKeySharePoints[1],
					big.NewInt(2),
				)
				messages[3].publicKeySharePoints[1] = new(bn256.G2).ScalarMult(
					messages[3].publicKeySharePoints[1],
					big.NewInt(2),
				)
			},
			expectedError:      nil,
			expectedAccusedIDs: []MemberID{2, 5},
		},
	}
	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			messages := make([]*MemberPublicKeySharePointsMessage, groupSize)

			for i, m := range sharingMembers {
				messages[i] = m.CalculatePublicKeySharePoints()
			}

			filteredMessages := filterMemberPublicKeySharePointsMessages(
				messages,
				sharingMember.ID,
			)

			if test.modifyPublicKeySharePointsMessages != nil {
				test.modifyPublicKeySharePointsMessages(filteredMessages)
			}

			accusedMessage, err := sharingMember.VerifyPublicKeySharePoints(filteredMessages)

			if !reflect.DeepEqual(test.expectedError, err) {
				t.Fatalf(
					"\nexpected: %s\nactual:   %s\n",
					test.expectedError,
					err,
				)
			}
			expectedAccusedMembersKeys := make(map[MemberID]*ephemeral.PrivateKey)
			for _, id := range test.expectedAccusedIDs {
				expectedAccusedMembersKeys[id] = sharingMember.ephemeralKeyPairs[id].PrivateKey
			}

			if !reflect.DeepEqual(accusedMessage.accusedMembersKeys, expectedAccusedMembersKeys) {
				t.Fatalf("incorrect accused IDs\nexpected: %v\nactual:   %v\n",
					expectedAccusedMembersKeys,
					accusedMessage.accusedMembersKeys,
				)
			}
		})
	}
}

func initializeQualifiedMembersGroup(threshold, groupSize int) (
	[]*QualifiedMember,
	error,
) {
	sharesJustifyingMembers, err := initializeSharesJustifyingMemberGroup(
		threshold,
		groupSize,
	)
	if err != nil {
		return nil, fmt.Errorf("group initialization failed [%s]", err)
	}

	var qualifiedMembers []*QualifiedMember
	for _, sjm := range sharesJustifyingMembers {
		qualifiedMembers = append(qualifiedMembers, sjm.InitializeQualified())
	}

	return qualifiedMembers, nil
}

func initializeSharingMembersGroup(threshold, groupSize int) (
	[]*SharingMember,
	error,
) {
	qualifiedMembers, err := initializeQualifiedMembersGroup(threshold, groupSize)
	if err != nil {
		return nil, fmt.Errorf("group initialization failed [%s]", err)
	}

	var sharingMembers []*SharingMember
	for _, sjm := range qualifiedMembers {
		sjm.secretCoefficients = make([]*big.Int, threshold+1)
		for i := 0; i < threshold+1; i++ {
			sjm.secretCoefficients[i], err = crand.Int(crand.Reader, bn256.Order)
			if err != nil {
				return nil, fmt.Errorf("secret share generation failed [%s]", err)
			}
		}
		sharingMembers = append(sharingMembers, sjm.InitializeSharing())
	}

	for _, sm := range sharingMembers {
		for _, sjm := range qualifiedMembers {
			sm.receivedValidSharesS[sjm.ID] = sjm.evaluateMemberShare(sm.ID, sjm.secretCoefficients)
		}
	}

	return sharingMembers, nil
}

func filterMemberPublicKeySharePointsMessages(
	messages []*MemberPublicKeySharePointsMessage, receiverID MemberID,
) []*MemberPublicKeySharePointsMessage {
	var result []*MemberPublicKeySharePointsMessage
	for _, msg := range messages {
		if msg.senderID != receiverID {
			result = append(result, msg)
		}
	}
	return result
}
