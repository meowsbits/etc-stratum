package proxy

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"

	"github.com/cyberpoolorg/etc-stratum/go-etchash"
	"github.com/ethereum/go-ethereum/common"
)

var ecip1099FBlockClassic uint64 = 11700000 // classic mainnet
var ecip1099FBlockMordor uint64 = 2520000   // mordor

var hasher *etchash.Etchash = nil

var (
	errUnknownNetworkConfig = errors.New("unknown network configuration")
	errStaleShare           = errors.New("stale share")
	errInvalidShare         = errors.New("invalid share")
	errValidShare           = errors.New("valid share")
	errBlockRejected        = errors.New("block rejected")
)

func (s *ProxyServer) processShare(login, id, ip string, t *BlockTemplate, params []string) (bool, error) {
	if hasher == nil {
		if s.config.Network == "classic" {
			hasher = etchash.New(&ecip1099FBlockClassic, nil)
		} else if s.config.Network == "mordor" {
			hasher = etchash.New(&ecip1099FBlockMordor, nil)
		} else {
			log.Printf("Unknown network configuration %s", s.config.Network)
			return false, fmt.Errorf(`%w: %s`, errUnknownNetworkConfig, s.config.Network)
		}
	}
	nonceHex := params[0]
	hashNoNonce := params[1]
	mixDigest := params[2]
	nonce, _ := strconv.ParseUint(strings.Replace(nonceHex, "0x", "", -1), 16, 64)
	shareDiff := s.config.Proxy.Difficulty

	h, ok := t.headers[hashNoNonce]
	if !ok {
		log.Printf("Stale share from %v@%v", login, ip)
		return false, errStaleShare
	}

	share := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  big.NewInt(shareDiff),
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	block := Block{
		number:      h.height,
		hashNoNonce: common.HexToHash(hashNoNonce),
		difficulty:  h.diff,
		nonce:       nonce,
		mixDigest:   common.HexToHash(mixDigest),
	}

	if !hasher.Verify(share) {
		return false, fmt.Errorf(`%w: hasher failed to verify`, errInvalidShare)
	}

	if hasher.Verify(block) {
		ok, err := s.rpc().SubmitBlock(params)
		if err != nil {
			log.Printf("Block submission failure at height %v for %v: %v", h.height, t.Header, err)
		} else if !ok {
			log.Printf("Block rejected at height %v for %v", h.height, t.Header)
			return false, errBlockRejected
		} else {
			s.fetchBlockTemplate()
			exist, err := s.backend.WriteBlock(login, id, params, shareDiff, h.diff.Int64(), h.height, s.hashrateExpiration)
			if exist {
				return true, errBlockRejected
			}
			if err != nil {
				log.Println("Failed to insert block candidate into backend:", err)
			} else {
				log.Printf("Inserted block %v to backend", h.height)
			}
			log.Printf("Block found by miner %v@%v at height %d", login, ip, h.height)
		}
	} else {
		log.Printf("Invalid block from %v@%v at height %v", login, ip, h.height)
		exist, err := s.backend.WriteShare(login, id, params, shareDiff, h.height, s.hashrateExpiration)
		if exist {
			return true, errValidShare
		}
		if err != nil {
			log.Println("Failed to insert share data into backend:", err)
		}
	}
	return false, nil
}
