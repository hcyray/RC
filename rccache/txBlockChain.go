package rccache

import (
	"fmt"
	"log"
	"math/rand"

	"github.com/boltdb/bolt"
	"github.com/uchihatmtkinu/RC/basic"
	"github.com/uchihatmtkinu/RC/treap"
)

//TxBlockChain is the blockchain database
type TxBlockChain struct {
	data    *bolt.DB
	lastTB  [32]byte
	USet    map[[32]byte]UTXOSet
	TXCache map[[32]byte]int
	AccData *gtreap.Treap
}

//NewBlockchain is to init the total chain
func (a *TxBlockChain) NewBlockchain() error {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return err
	}
	defer a.data.Close()

	err = a.data.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TBBucket))

		if b == nil {
			genesis := basic.NewGensisTxBlock()
			b, err := tx.CreateBucket([]byte(TBBucket))
			var tmp []byte
			err = genesis.Decode(&tmp)
			if err != nil {
				return nil
			}
			err = b.Put(append([]byte("B"), genesis.HashID[:]...), tmp)
			err = b.Put([]byte("XB"), genesis.HashID[:])
			a.lastTB = genesis.HashID
		} else {
			copy(a.lastTB[:], b.Get([]byte("XB"))[:32])
		}
		b = tx.Bucket([]byte(ACCBucket))
		a.AccData = gtreap.NewTreap(byteCompare)
		if b == nil {
			_, err := tx.CreateBucket([]byte(ACCBucket))
			if err != nil {
				log.Panic(err)
			}
		} else {
			c := b.Cursor()
			var tmp *basic.AccCache
			for k, v := c.First(); k != nil; k, v = c.Next() {
				tmp = new(basic.AccCache)
				copy(tmp.ID[:], k[:32])
				tmpStr := v
				basic.DecodeInt(&tmpStr, &tmp.Value)
				a.AccData = a.AccData.Upsert(tmp, rand.Int())
			}
		}
		b = tx.Bucket([]byte(UTXOBucket))
		if b == nil {
			_, err := tx.CreateBucket([]byte(UTXOBucket))
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
	return err
}

//AddBlock is adding a new txblock
func (a *TxBlockChain) AddBlock(x *basic.TxBlock) error {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	var lastHash [32]byte
	err = a.data.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TBBucket))
		copy(lastHash[:], b.Get([]byte("lTB"))[:32])

		return nil
	})
	if lastHash != x.PrevHash {
		return fmt.Errorf("Failed to add TxBlock: PrevHash not match")
	}
	err = a.data.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TBBucket))
		err := b.Put(append([]byte("B"), x.HashID[:]...), x.Serial())
		if err != nil {
			return err
		}
		err = b.Put([]byte("XB"), x.HashID[:])
		if err != nil {
			return err
		}
		a.lastTB = x.HashID

		return nil
	})
	return nil
}

//LatestTxBlock return the highest txblock
func (a *TxBlockChain) LatestTxBlock() *basic.TxBlock {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	var tmpStr []byte
	err = a.data.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(TBBucket))
		tmpStr = b.Get(append([]byte("B"), a.lastTB[:]...))

		return nil
	})
	var tmp basic.TxBlock
	err = tmp.Decode(&tmpStr)
	if err != nil {
		return nil
	}
	return &tmp
}

//AddAccount is adding a new account or update
func (a *TxBlockChain) AddAccount(x *basic.AccCache) error {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	err = a.data.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(ACCBucket))
		if x.Value == 0 {
			err := b.Delete(x.ID[:])
			return err
		}
		var tmp []byte
		basic.EncodeInt(&tmp, x.Value)
		err := b.Put(x.ID[:], tmp)
		return err
	})
	if err != nil {
		return err
	}
	return nil
}

//CheckUTXO is to check whether the utxo is available
func (a *TxBlockChain) CheckUTXO(x *basic.InType, h [32]byte) bool {
	if x.Acc() {
		tmp := a.FindAcc(x.PrevTx)
		return tmp.Value >= x.Index
	}
	tmp, ok := a.USet[x.PrevTx]
	res := false
	if !ok {
		var err error
		a.data, err = bolt.Open(dbFile, 0600, nil)
		if err != nil {
			log.Panic(err)
		}
		defer a.data.Close()
		err = a.data.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(UTXOBucket))
			tmpStr := b.Get(x.PrevTx[:])
			if tmpStr == nil {
				return nil
			}
			res = true
			err = tmp.Decode(&tmpStr)
			if err != nil {
				return fmt.Errorf("Decoding error")
			}
			return nil
		})
		if !res {
			return false
		}
		if x.Index >= tmp.Cnt {
			return false
		}
		a.USet[x.PrevTx] = tmp
	}
	if x.Index >= tmp.Cnt {
		return false
	}
	if tmp.Stat[x.Index] != 0 {
		return false
	}
	return x.VerifyIn(&tmp.Data[x.Index], h)
}

//LockUTXO is to lock the value
func (a *TxBlockChain) LockUTXO(x *basic.InType) error {
	if x.Acc() {
		tmp := a.FindAcc(x.PrevTx)
		tmp.Value -= x.Index
	} else {
		tmp, ok := a.USet[x.PrevTx]
		if !ok || x.Index >= tmp.Cnt || tmp.Stat[x.Index] != 0 {
			return fmt.Errorf("Locking utxo failed")
		}
		tmp.Stat[x.Index] = 2
	}
	return nil
}

//UnlockUTXO is to lock the value
func (a *TxBlockChain) UnlockUTXO(x *basic.InType) error {
	if x.Acc() {
		tmp := a.FindAcc(x.PrevTx)
		tmp.Value += x.Index
	} else {
		tmp, ok := a.USet[x.PrevTx]
		if !ok || x.Index >= tmp.Cnt || tmp.Stat[x.Index] != 2 {
			return fmt.Errorf("Unlocking utxo failed")
		}
		tmp.Stat[x.Index] = 0
	}
	return nil
}

//ConfirmUTXO is to make the value used
func (a *TxBlockChain) ConfirmUTXO(x *basic.InType) error {
	if !x.Acc() {
		tmp, ok := a.USet[x.PrevTx]
		if !ok || x.Index >= tmp.Cnt || tmp.Stat[x.Index] != 2 {
			return fmt.Errorf("Confirming utxo failed")
		}
		tmp.Stat[x.Index] = 1
	}
	return nil
}

//MakeFinalTx generates the final blocks transactions
func (a *TxBlockChain) MakeFinalTx() *[]basic.Transaction {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	res := make([]basic.Transaction, 0, 10000)
	tmpMap := make(map[[32]byte]uint32)
	tmp := a.AccData.Min()
	a.AccData.VisitAscend(tmp, func(i gtreap.Item) bool {
		var tmpTx basic.Transaction
		tmpIn := basic.InType{PrevTx: i.(*basic.AccCache).ID, Index: i.(*basic.AccCache).Value}
		var tmpOut basic.OutType
		tmpOut.Value = i.(*basic.AccCache).Value
		tmpOut.Address = i.(*basic.AccCache).ID
		tmpTx.New(1)
		tmpTx.AddIn(tmpIn)
		tmpTx.AddOut(tmpOut)
		res = append(res, tmpTx)
		tmpMap[tmpOut.Address] = uint32(len(res) - 1)
		return true
	})

	err = a.data.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(UTXOBucket))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			tmp := new(UTXOSet)
			tmpStr := v
			var tmpHash [32]byte
			copy(tmpHash[:], k[:32])
			err := tmp.Decode(&tmpStr)
			if err != nil {
				continue
			}
			for i := uint32(0); i < tmp.Cnt; i++ {
				if tmp.Stat[i] == 0 {
					tmpID, ok := tmpMap[tmp.Data[i].Address]
					if ok {
						tmpIn := basic.InType{PrevTx: tmpHash, Index: i}
						res[tmpID].AddIn(tmpIn)
						res[tmpID].Out[0].Value += tmp.Data[i].Value
					} else {
						var tmpTx basic.Transaction
						tmpIn := basic.InType{PrevTx: tmpHash, Index: i}
						var tmpOut basic.OutType
						tmpOut.Value = tmp.Data[i].Value
						tmpOut.Address = tmp.Data[i].Address
						tmpTx.New(1)
						tmpTx.AddIn(tmpIn)
						tmpTx.AddOut(tmpOut)
						res = append(res, tmpTx)
						tmpMap[tmp.Data[i].Address] = uint32(len(res) - 1)
					}
				}
			}
		}
		return nil
	})
	return &res
}

//UpdateFinal is to update the final block
func (a *TxBlockChain) UpdateFinal(x *basic.TxBlock) error {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	for i := uint32(0); i < x.TxCnt; i++ {
		tmp := a.FindAcc(x.TxArray[i].Out[0].Address)
		if tmp != nil {
			tmp.Value = x.TxArray[i].Out[0].Value
		} else {
			tmp = new(basic.AccCache)
			tmp.ID = x.TxArray[i].Out[0].Address
			tmp.Value = x.TxArray[i].Out[0].Value
			a.AccData.Upsert(tmp, rand.Int())
		}
	}

	err = a.data.Update(func(tx *bolt.Tx) error {
		tx.DeleteBucket([]byte(UTXOBucket))
		tx.CreateBucket([]byte(UTXOBucket))
		b := tx.Bucket([]byte(ACCBucket))
		tmp := a.AccData.Min()
		a.AccData.VisitAscend(tmp, func(i gtreap.Item) bool {
			if i.(*basic.AccCache).Value == 0 {
				b.Delete(i.(*basic.AccCache).ID[:])
			} else {
				var tmp []byte
				basic.EncodeInt(&tmp, i.(*basic.AccCache).Value)
				b.Put(i.(*basic.AccCache).ID[:], tmp)
			}
			return true
		})
		return nil
	})
	return err
}

//UpdateUTXO is to update utxo set
func (a *TxBlockChain) UpdateUTXO(x *basic.TxBlock, shardindex uint32) error {
	var err error
	a.data, err = bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}
	defer a.data.Close()
	err = a.data.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(UTXOBucket))
		for i := uint32(0); i < x.TxCnt; i++ {
			_, oktx := a.TXCache[x.TxArray[i].Hash]
			for j := uint32(0); j < x.TxArray[i].TxinCnt; j++ {
				if x.TxArray[i].In[j].ShardIndex() == shardindex {
					if x.TxArray[i].In[j].Acc() {
						tmp := a.FindAcc(x.TxArray[i].In[j].PrevTx)
						if tmp != nil && !oktx {
							tmp.Value -= x.TxArray[i].In[j].Index
						}
					} else {
						tmp, ok := a.USet[x.TxArray[i].In[j].PrevTx]
						if ok {
							if tmp.Stat[x.TxArray[i].In[j].Index] != 1 {
								tmp.Stat[x.TxArray[i].In[j].Index] = 1
								tmp.Remain--
								if tmp.Remain == 0 {
									delete(a.USet, x.TxArray[i].In[j].PrevTx)
									b.Delete(x.TxArray[i].In[j].PrevTx[:])
								} else {
									a.USet[x.TxArray[i].In[j].PrevTx] = tmp
								}
							}
						}
						tmpStr := b.Get(x.TxArray[i].In[j].PrevTx[:])
						err := tmp.Decode(&tmpStr)
						if err == nil && tmp.Stat[x.TxArray[i].In[j].Index] != 1 {
							tmp.Stat[x.TxArray[i].In[j].Index] = 1
							tmp.Remain--
							if tmp.Remain == 0 {
								b.Delete(x.TxArray[i].In[j].PrevTx[:])
							} else {
								tmpStr = tmp.Encode()
								b.Put(x.TxArray[i].In[j].PrevTx[:], tmpStr)
							}
						}
					}
				}
			}
			tmp := UTXOSet{Cnt: x.TxArray[i].TxoutCnt}
			copy(tmp.Data, x.TxArray[i].Out)
			tmp.Remain = tmp.Cnt
			tmp.Stat = make([]uint32, tmp.Cnt)
			for i := uint32(0); i < tmp.Cnt; i++ {
				if tmp.Data[i].ShardIndex() != shardindex {
					tmp.Stat[i] = 1
					tmp.Remain--
				}
			}
			if tmp.Remain > 0 {
				b.Put(x.TxArray[i].Hash[:], tmp.Encode())
			}
			if oktx {
				delete(a.TXCache, x.TxArray[i].Hash)
			}
		}
		return nil
	})
	return err

}