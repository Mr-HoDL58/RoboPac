package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kehiy/RoboPac/client"
	"github.com/kehiy/RoboPac/log"
	"github.com/kehiy/RoboPac/nowpayments"
	rpstore "github.com/kehiy/RoboPac/store"
	"github.com/kehiy/RoboPac/twitter_api"
	"github.com/kehiy/RoboPac/utils"
	"github.com/kehiy/RoboPac/wallet"
	"github.com/libp2p/go-libp2p/core/peer"
	pactus "github.com/pactus-project/pactus/www/grpc/gen/go"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

var peerID, _ = peer.Decode("12D3KooWNwudyHVEwtyRTkTx9JoWgHo65hkPUxU12pKviAreVJYg")

var networkInfo = &pactus.GetNetworkInfoResponse{
	ConnectedPeersCount: 5,
	NetworkName:         "test",
	ConnectedPeers: []*pactus.PeerInfo{
		{
			ConsensusKeys:    []string{"pub-key", "mainnet-addr-fail-not-first-validator"},
			ConsensusAddress: []string{"valid-address2", "mainnet-addr-fail-not-first-validator"},
		},
		{
			ConsensusKeys:    []string{"pubKey-3", "pubKey-4"},
			ConsensusAddress: []string{"addr-3", "addr-4"},
		},
		{
			ConsensusKeys:    []string{"pub-key", "pubKey-4"},
			ConsensusAddress: []string{"valid-address", "addr-4"},
			Agent:            "node=pactus-gui.exe/node-version=v0.20.0/protocol-version=1/os=windows/arch=amd64",
			PeerId:           []byte(peerID),
			Address:          "/ip4/000.000.000.000/tcp/21777",
		},
		{
			ConsensusKeys:    []string{"public-key"},
			ConsensusAddress: []string{"mainnet-addr"},
		},
		{
			ConsensusAddress: []string{"invalid-address", "invalid-address-2"},
		},
		{
			ConsensusAddress: []string{"mainnet-addr-fail-empty-tx-hash"},
			ConsensusKeys:    []string{"public-key-fail-empty-tx-hash"},
		},
		{
			ConsensusAddress: []string{"mainnet-addr-panic-add-claimer-failed"},
			ConsensusKeys:    []string{"public-key-panic-add-claimer-failed"},
		},
		{
			ConsensusAddress: []string{"addr"},
			ConsensusKeys:    []string{"public-key"},
		},
	},
}

func setup(t *testing.T) (*BotEngine, *client.MockIClient, *rpstore.MockIStore,
	*wallet.MockIWallet, *twitter_api.MockIClient, *nowpayments.MockINowpayment, context.Context,
) {
	t.Helper()
	ctrl := gomock.NewController(t)

	// mocking client manager.
	sl := log.NewSubLogger("test")
	mockClient := client.NewMockIClient(ctrl)

	ctx, cancel := context.WithCancel(context.Background())
	cm := client.NewClientMgr(ctx)
	cm.AddClient(mockClient)

	mockClient.EXPECT().GetNetworkInfo(ctx).Return(
		networkInfo, nil,
	)

	cm.Start()

	mockWallet := wallet.NewMockIWallet(ctrl)
	mockStore := rpstore.NewMockIStore(ctrl)
	mockTwitter := twitter_api.NewMockIClient(ctrl)
	mockNowPayments := nowpayments.NewMockINowpayment(ctrl)

	eng := newBotEngine(sl, cm, mockWallet, mockStore, mockTwitter, mockNowPayments, []string{""}, ctx, cancel)
	return eng, mockClient, mockStore, mockWallet, mockTwitter, mockNowPayments, ctx
}

func TestNetworkStatus(t *testing.T) {
	eng, client, _, _, _, _, ctx := setup(t)

	client.EXPECT().GetNetworkInfo(ctx).Return(
		&pactus.GetNetworkInfoResponse{
			ConnectedPeersCount: 5,
			NetworkName:         "test",
		}, nil,
	)

	client.EXPECT().GetBlockchainInfo(ctx).Return(
		&pactus.GetBlockchainInfoResponse{
			TotalPower:      1234,
			LastBlockHeight: 150,
			TotalAccounts:   158,
		}, nil,
	).AnyTimes()

	client.EXPECT().GetBalance(ctx, "pc1z2r0fmu8sg2ffa0tgrr08gnefcxl2kq7wvquf8z").Return(
		int64(100), nil,
	)

	client.EXPECT().GetBalance(ctx, "pc1zprhnvcsy3pthekdcu28cw8muw4f432hkwgfasv").Return(
		int64(100), nil,
	)

	client.EXPECT().GetBalance(ctx, "pc1znn2qxsugfrt7j4608zvtnxf8dnz8skrxguyf45").Return(
		int64(100), nil,
	)

	client.EXPECT().GetBalance(ctx, "pc1zs64vdggjcshumjwzaskhfn0j9gfpkvche3kxd3").Return(
		int64(100), nil,
	)

	client.EXPECT().GetBalance(ctx, "pc1zuavu4sjcxcx9zsl8rlwwx0amnl94sp0el3u37g").Return(
		int64(100), nil,
	)

	client.EXPECT().GetBalance(ctx, "pc1zf0gyc4kxlfsvu64pheqzmk8r9eyzxqvxlk6s6t").Return(
		int64(100), nil,
	)

	status, err := eng.NetworkStatus()
	assert.NoError(t, err)

	assert.Equal(t, uint32(5), status.ConnectedPeersCount)
	assert.Equal(t, "test", status.NetworkName)
	assert.Equal(t, int64(1234), status.TotalNetworkPower)
}

func TestNetworkHealth(t *testing.T) {
	eng, client, _, _, _, _, ctx := setup(t)

	t.Run("should be healthy", func(t *testing.T) {
		currentTime := time.Now().Unix()
		client.EXPECT().LastBlockTime(ctx).Return(uint32(currentTime), uint32(100), nil)

		time.Sleep(2 * time.Second)

		healthy, err := eng.NetworkHealth()
		assert.NoError(t, err)

		assert.Equal(t, true, healthy.HealthStatus)
		assert.Equal(t, uint32(100), healthy.LastBlockHeight)
		assert.Equal(t, currentTime, healthy.LastBlockTime.Unix())
		assert.Equal(t, currentTime+2, healthy.CurrentTime.Unix())
		assert.Equal(t, int64(2), healthy.TimeDifference)
	})

	t.Run("should be unhealthy", func(t *testing.T) {
		currentTime := time.Now().Unix() - 16 // time difference is more than 15 seconds.
		client.EXPECT().LastBlockTime(ctx).Return(uint32(currentTime), uint32(100), nil)

		healthy, err := eng.NetworkHealth()
		assert.NoError(t, err)

		assert.Equal(t, false, healthy.HealthStatus)
	})
}

func TestNodeInfo(t *testing.T) {
	eng, client, _, _, _, _, ctx := setup(t)
	t.Run("should work, valid address", func(t *testing.T) {
		valAddress := "valid-address"
		pubKey := "pub-key"

		peerID, err := peer.Decode("12D3KooWNwudyHVEwtyRTkTx9JoWgHo65hkPUxU12pKviAreVJYg")
		assert.NoError(t, err)

		client.EXPECT().GetNetworkInfo(ctx).Return(
			&pactus.GetNetworkInfoResponse{
				ConnectedPeers: []*pactus.PeerInfo{
					{
						ConsensusKeys:    []string{pubKey},
						ConsensusAddress: []string{valAddress},
						Height:           100,
						PeerId:           []byte(peerID),
						Agent:            "node=pactus-gui.exe/node-version=v0.20.0/protocol-version=1/os=windows/arch=amd64",
						Address:          "/ip4/000.000.000.000/tcp/21777",
					},
					{
						ConsensusKeys:    []string{pubKey},
						ConsensusAddress: []string{valAddress},
					},
				},
			}, nil,
		).AnyTimes()

		client.EXPECT().GetValidatorInfo(ctx, valAddress).Return(
			&pactus.GetValidatorResponse{
				Validator: &pactus.ValidatorInfo{
					PublicKey:         pubKey,
					Stake:             int64(1_000),
					Address:           valAddress,
					Number:            1,
					AvailabilityScore: 0.9,
				},
			}, nil,
		).AnyTimes()

		info, err := eng.NodeInfo(valAddress)
		assert.NoError(t, err)

		assert.Equal(t, int64(1_000), info.StakeAmount)
		assert.Equal(t, float64(0.9), info.AvailabilityScore)
	})
}

func TestClaim(t *testing.T) {
	t.Run("everything normal and good", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr"
		testnetAddr := "testnet-addr"
		pubKey := "public-key"
		discordID := "123456789"
		amount := int64(30)
		memo := "TestNet reward claim from RoboPac"
		txID := "tx-id"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		).MaxTimes(2)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				TotalReward: amount,
				ClaimedTxID: "",
			},
		)

		wallet.EXPECT().BondTransaction(pubKey, mainnetAddr, memo, amount).Return(
			txID, nil,
		).MaxTimes(1)

		store.EXPECT().AddClaimTransaction(testnetAddr, txID).Return(
			nil,
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.NoError(t, err)
		assert.NotNil(t, expectedTx, txID)

		//! can't claim twice immediately before transaction is committed:
		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		).Times(1)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				TotalReward: amount,
				ClaimedTxID: txID,
			},
		).Times(1)

		expectedTx, err = eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.Error(t, err)
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, already staked", func(t *testing.T) {
		eng, client, _, _, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-balance"
		testnetAddr := "testnet-addr-fail-balance"
		discordID := "123456789-already staked"

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			&pactus.GetValidatorResponse{
				Validator: &pactus.ValidatorInfo{
					Stake: 1,
				},
			}, nil,
		).Times(1)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "this address is already a staked validator")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, low balance", func(t *testing.T) {
		eng, client, _, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-balance"
		testnetAddr := "testnet-addr-fail-balance"
		discordID := "123456789-fail-balance"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(499),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)
		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "insufficient wallet balance")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, claimer not found", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-notfound"
		testnetAddr := "testnet-addr-fail-notfound"
		discordID := "123456789-fail-notfound"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			nil,
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "claimer not found")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, different Discord ID", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-different-id"
		testnetAddr := "testnet-addr-fail-different-id"
		discordID := "123456789-fail-different-id"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID: "invalid-discord-id",
			},
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "invalid claimer")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, not first validator address", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-not-first-validator"
		testnetAddr := "testnet-addr-fail-not-first-validator"
		discordID := "123456789-fail-not-first-validator"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				ClaimedTxID: "",
			},
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "please enter the first validator address")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, validator not found", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-validator-not-found"
		testnetAddr := "testnet-addr-fail-validator-not-found"
		discordID := "123456789-fail-validator-not-found"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				ClaimedTxID: "",
			},
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "peer does not exist with this address: mainnet-addr-fail-validator-not-found")
		assert.Empty(t, expectedTx)
	})

	t.Run("should fail, empty transaction hash", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-fail-empty-tx-hash"
		testnetAddr := "testnet-addr-fail-empty-tx-hash"
		pubKey := "public-key-fail-empty-tx-hash"
		discordID := "123456789-fail-empty-tx-hash"
		amount := int64(30)
		memo := "TestNet reward claim from RoboPac"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				TotalReward: amount,
				ClaimedTxID: "",
			},
		)

		wallet.EXPECT().BondTransaction(pubKey, mainnetAddr, memo, amount).Return(
			"", nil,
		)

		expectedTx, err := eng.Claim(discordID, testnetAddr, mainnetAddr)
		assert.EqualError(t, err, "can't send bond transaction")
		assert.Empty(t, expectedTx)
	})

	t.Run("should panic, add claimer failed", func(t *testing.T) {
		eng, client, store, wallet, _, _, ctx := setup(t)

		mainnetAddr := "mainnet-addr-panic-add-claimer-failed"
		testnetAddr := "testnet-addr-panic-add-claimer-failed"
		pubKey := "public-key-panic-add-claimer-failed"
		discordID := "123456789-panic-add-claimer-failed"
		amount := int64(30)
		memo := "TestNet reward claim from RoboPac"
		txID := "tx-id-panic-add-claimer-failed"

		wallet.EXPECT().Balance().Return(
			utils.CoinToChange(501),
		)

		client.EXPECT().GetValidatorInfo(ctx, mainnetAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		store.EXPECT().ClaimerInfo(testnetAddr).Return(
			&rpstore.Claimer{
				DiscordID:   discordID,
				TotalReward: amount,
				ClaimedTxID: "",
			},
		)

		wallet.EXPECT().BondTransaction(pubKey, mainnetAddr, memo, amount).Return(
			txID, nil,
		)

		store.EXPECT().AddClaimTransaction(testnetAddr, txID).Return(
			errors.New(""),
		)

		assert.Panics(t, func() {
			_, _ = eng.Claim(discordID, testnetAddr, mainnetAddr)
		})
	})
}

func TestBoosterProgram(t *testing.T) {
	t.Run("staked validator", func(t *testing.T) {
		eng, client, store, _, _, _, ctx := setup(t)

		twitterName := "anything"
		discordID := "123456789"
		valAddr := "staked-validator"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			&pactus.GetValidatorResponse{}, nil,
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.Error(t, err)
	})

	t.Run("non-existing validator", func(t *testing.T) {
		eng, client, store, _, _, _, ctx := setup(t)

		twitterName := "anything"
		discordID := "123456789"
		valAddr := "non-existing-validator"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.Error(t, err)
	})

	t.Run("non-existing twitter account", func(t *testing.T) {
		eng, client, store, _, twitter, _, ctx := setup(t)

		twitterName := "non-existing-twitter"
		discordID := "123456789"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		expectedErr := errors.New("not exists")
		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			nil, expectedErr,
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("not old enough", func(t *testing.T) {
		eng, client, store, _, twitter, _, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			false,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterID: twitterID,
				CreatedAt: time.Now().AddDate(-1, 0, 0),
			}, nil,
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.Error(t, err)
	})

	t.Run("less than 200 followers", func(t *testing.T) {
		eng, client, store, _, twitter, _, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			false,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterID: twitterID,
				CreatedAt: time.Now().AddDate(-4, 0, 0),
				Followers: 100,
			}, nil,
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.Error(t, err)
	})

	t.Run("active account, but not retweeted", func(t *testing.T) {
		eng, client, store, _, twitter, _, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			false,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterID:  twitterID,
				CreatedAt:  time.Now().AddDate(-4, 0, 0),
				Followers:  300,
				IsVerified: false,
			}, nil,
		)

		twitter.EXPECT().RetweetSearch(eng.ctx, discordID, twitterName).Return(
			nil, fmt.Errorf("not found"),
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.Error(t, err)
	})

	t.Run("active account, less that 1000 followers", func(t *testing.T) {
		eng, client, store, _, twitter, nowPayments, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			false,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterName: twitterName,
				TwitterID:   twitterID,
				CreatedAt:   time.Now().AddDate(-4, 0, 0),
				Followers:   300,
				IsVerified:  false,
			}, nil,
		)

		twitter.EXPECT().RetweetSearch(eng.ctx, discordID, twitterName).Return(
			&twitter_api.TweetInfo{
				CreatedAt: time.Now().AddDate(0, 0, -2),
			}, nil,
		)

		nowPayments.EXPECT().CreatePayment(gomock.Any()).Return(
			nil,
		)

		store.EXPECT().SaveTwitterParty(gomock.Any()).Return(
			nil,
		)

		party, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.NoError(t, err)

		assert.Equal(t, int64(150), party.AmountInPAC)
		assert.Equal(t, 40, party.TotalPrice)
		assert.Equal(t, twitterName, party.TwitterName)
		assert.Equal(t, twitterID, party.TwitterID)
		assert.Equal(t, valAddr, party.ValAddr)
	})

	t.Run("active account, more that 1000 followers", func(t *testing.T) {
		eng, client, store, _, twitter, nowPayments, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 99,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			false,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterName: twitterName,
				TwitterID:   twitterID,
				CreatedAt:   time.Now().AddDate(-4, 0, 0),
				Followers:   1001,
				IsVerified:  false,
			}, nil,
		)

		twitter.EXPECT().RetweetSearch(eng.ctx, discordID, twitterName).Return(
			&twitter_api.TweetInfo{
				CreatedAt: time.Now().AddDate(0, 0, -2),
			}, nil,
		)

		nowPayments.EXPECT().CreatePayment(gomock.Any()).Return(
			nil,
		)

		store.EXPECT().SaveTwitterParty(gomock.Any()).Return(
			nil,
		)

		party, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.NoError(t, err)

		assert.Equal(t, int64(200), party.AmountInPAC)
		assert.Equal(t, 30, party.TotalPrice)
		assert.Equal(t, twitterName, party.TwitterName)
		assert.Equal(t, twitterID, party.TwitterID)
		assert.Equal(t, valAddr, party.ValAddr)
	})

	t.Run("verified account", func(t *testing.T) {
		eng, client, store, _, twitter, nowPayments, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 400,
			},
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterID:   twitterID,
				TwitterName: twitterName,
				CreatedAt:   time.Now().AddDate(-1, 0, 0),
				Followers:   100,
				IsVerified:  true,
			}, nil,
		)

		twitter.EXPECT().RetweetSearch(eng.ctx, discordID, twitterName).Return(
			&twitter_api.TweetInfo{
				CreatedAt: time.Now().AddDate(0, 0, -2),
			}, nil,
		)

		nowPayments.EXPECT().CreatePayment(gomock.Any()).Return(
			nil,
		)

		store.EXPECT().SaveTwitterParty(gomock.Any()).Return(
			nil,
		)

		p, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.NoError(t, err)

		assert.Equal(t, 50, p.TotalPrice)
	})

	t.Run("whitelisted account", func(t *testing.T) {
		eng, client, store, _, twitter, nowPayments, ctx := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		twitterID := "1234"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 100,
			},
		)

		store.EXPECT().IsWhitelisted(twitterID).Return(
			true,
		)

		store.EXPECT().FindTwitterParty(twitterName).Return(
			nil,
		)

		client.EXPECT().GetValidatorInfo(ctx, valAddr).Return(
			nil, fmt.Errorf("not found"),
		)

		twitter.EXPECT().UserInfo(eng.ctx, twitterName).Return(
			&twitter_api.UserInfo{
				TwitterID:   twitterID,
				TwitterName: twitterName,
				CreatedAt:   time.Now().AddDate(-1, 0, 0),
				Followers:   100,
				IsVerified:  false,
			}, nil,
		)

		twitter.EXPECT().RetweetSearch(eng.ctx, discordID, twitterName).Return(
			&twitter_api.TweetInfo{
				CreatedAt: time.Now().AddDate(0, 0, -2),
			}, nil,
		)

		nowPayments.EXPECT().CreatePayment(gomock.Any()).Return(
			nil,
		)

		store.EXPECT().SaveTwitterParty(gomock.Any()).Return(
			nil,
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.NoError(t, err)
	})

	t.Run("program end", func(t *testing.T) {
		eng, _, store, _, _, _, _ := setup(t)

		twitterName := "abcd"
		discordID := "123456789"
		valAddr := "addr"

		store.EXPECT().BoosterStatus().Return(
			&rpstore.BoosterStatus{
				AllPkgs: 501,
			},
		)

		_, err := eng.BoosterPayment(discordID, twitterName, valAddr)
		assert.EqualError(t, err, "program is finished")
	})
}

func TestBoosterPrice(t *testing.T) {
	for i := 0; i < 501; i++ {
		if i < 100 {
			price := boosterPrice(i)
			assert.Equal(t, 30, price)
		}

		if i > 100 && i < 200 {
			price := boosterPrice(i)
			assert.Equal(t, 40, price)
		}

		if i > 200 {
			price := boosterPrice(i)
			assert.Equal(t, 50, price)
		}
	}
}
