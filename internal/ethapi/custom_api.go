package ethapi

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

func (s *PublicBlockChainAPI) GetFullBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, number)
	if err != nil || block == nil {
		return nil, err
	}
	response, err := s.rpcMarshalBlock(ctx, block, true, fullTx)
	if err != nil {
		return nil, err
	}
	// Pending blocks need to nil out a few fields
	if number == rpc.PendingBlockNumber {
		for _, field := range []string{"hash", "nonce", "miner"} {
			response[field] = nil
		}
	}
	if !fullTx {
		return response, err
	}
	txs := response["transactions"].([]interface{})
	// build map with sender for every tx hash
	txByHash := make(map[common.Hash]*RPCTransaction)
	for _, rawTx := range txs {
		tx, ok := rawTx.(*RPCTransaction)
		if !ok {
			return nil, fmt.Errorf("can't get RPC tx: %s", tx.Hash.Hex())
		}
		txByHash[tx.Hash] = tx
	}
	response["receipts"], err = s.getBlockReceipts(ctx, txByHash, block)
	return response, err
}

func (s *PublicBlockChainAPI) getBlockReceipts(ctx context.Context, txByHash map[common.Hash]*RPCTransaction, block *types.Block) ([]map[string]interface{}, error) {
	// get all block receipts from database
	receipts, err := s.b.GetReceipts(ctx, block.Hash())
	if err != nil {
		return nil, err
	}
	// build result array with packed receipts
	result := make([]map[string]interface{}, 0)
	for _, receipt := range receipts {
		tx, ok := txByHash[receipt.TxHash]
		if !ok {
			log.Error("Can't find transaction sender in cache", "hash", receipt.TxHash)
			continue
		}
		fields := map[string]interface{}{
			"blockHash":         block.Hash(),
			"blockNumber":       "0x" + block.Number().Text(16),
			"transactionHash":   tx.Hash,
			"transactionIndex":  tx.TransactionIndex,
			"from":              tx.From,
			"to":                tx.To,
			"gasUsed":           hexutil.Uint64(receipt.GasUsed),
			"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
			"contractAddress":   nil,
			"logs":              receipt.Logs,
			"logsBloom":         receipt.Bloom,
			"type":              tx.Type,
		}
		if len(receipt.PostState) > 0 {
			fields["root"] = hexutil.Bytes(receipt.PostState)
		} else {
			fields["status"] = hexutil.Uint(receipt.Status)
		}
		if receipt.Logs == nil {
			fields["logs"] = [][]*types.Log{}
		}
		if receipt.ContractAddress != (common.Address{}) {
			fields["contractAddress"] = receipt.ContractAddress
		}
		result = append(result, fields)
	}
	return result, nil
}
