package Reputation

import (

	"github.com/boltdb/bolt"
	"log"
	"os"
	"fmt"
	"github.com/uchihatmtkinu/RC/shard"
	"github.com/uchihatmtkinu/RC/rccache"
	"strconv"
)

const dbFile = "RepBlockchain"
const blocksBucket = "blocks"

//reputation block chain
type RepBlockchain struct {
	Tip [32]byte
	Db *bolt.DB
}

// RepBlockchainIterator is used to iterate over Repblockchain blocks
type RepBlockchainIterator struct {
	currentHash [32]byte
	db          *bolt.DB
}

// MineRepBlock mines a new repblock with the provided transactions
func (bc *RepBlockchain) MineRepBlock(ms *[]shard.MemShard, cache *rccache.DbRef) {
	var lastHash [32]byte
	var fromOtherFlag bool

	CurrentRepBlock.Mu.RLock()
	lastHash = CurrentRepBlock.Block.Hash
	CurrentRepBlock.Mu.RUnlock()

	tmp := [][32]byte{{0}}
	cache.TBCache = &tmp

	CurrentRepBlock.Mu.Lock()
	defer CurrentRepBlock.Mu.Unlock()
	CurrentRepBlock.Block, fromOtherFlag = NewRepBlock(ms, shard.StartFlag,  shard.PreviousSyncBlockHash, *(cache.TBCache) ,lastHash)
	CurrentRepBlock.Round ++
	if fromOtherFlag {
		RepPowTxCh <- RepPowInfo{CurrentRepBlock.Round, CurrentRepBlock.Block.Nonce, CurrentRepBlock.Block.Hash}
	}
	shard.StartFlag = false

	cache.TBCache = nil
	err := bc.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(CurrentRepBlock.Block.Hash[:], CurrentRepBlock.Block.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("lb"), CurrentRepBlock.Block.Hash[:])
		if err != nil {
			log.Panic(err)
		}

		bc.Tip = CurrentRepBlock.Block.Hash

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// add a new syncBlock on RepBlockChain
func (bc *RepBlockchain) AddSyncBlock(ms *[]shard.MemShard, CoSignature []byte) {
	var lastRepBlockHash [32]byte
	tmpCoSignature := make([]byte, len(CoSignature))
	copy(tmpCoSignature, CoSignature)
	//var prevSyncBlockHash [][32]byte
	CurrentRepBlock.Mu.RLock()
	lastRepBlockHash = CurrentRepBlock.Block.Hash
	CurrentRepBlock.Mu.RUnlock()


	CurrentSyncBlock.Mu.Lock()
	CurrentSyncBlock.Block = NewSynBlock(ms, shard.PreviousSyncBlockHash, lastRepBlockHash,  tmpCoSignature)
	CurrentSyncBlock.Epoch ++
	shard.PreviousSyncBlockHash[shard.MyMenShard.Shard] = CurrentSyncBlock.Block.Hash
	defer CurrentSyncBlock.Mu.Unlock()
	err := bc.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(CurrentSyncBlock.Block.Hash[:], CurrentSyncBlock.Block.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("lsb"+strconv.FormatInt(int64(shard.MyMenShard.Shard), 10)), CurrentSyncBlock.Block.Hash[:])
		if err != nil {
			log.Panic(err)
		}

		bc.Tip = CurrentSyncBlock.Block.Hash

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

}

//AddSyncBlockFromOtherShards add sync block from k-th shard
func (bc *RepBlockchain) AddSyncBlockFromOtherShards(syncBlock *SyncBlock, k int) {
	err := bc.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(syncBlock.Hash[:], syncBlock.Serialize())
		if err != nil {
			log.Panic(err)
		}
		fmt.Println("Add from other")
		CurrentSyncBlock.Mu.RLock()
		CurrentSyncBlock.Block.Print()
		CurrentSyncBlock.Mu.RUnlock()
		shard.PreviousSyncBlockHash[k] = syncBlock.Hash
		fmt.Println("Add from other after")
		CurrentSyncBlock.Mu.RLock()
		CurrentSyncBlock.Block.Print()
		CurrentSyncBlock.Mu.RUnlock()
		/*
		err = b.Put([]byte("lsb"+strconv.FormatInt(int64(k), 10)), syncBlock.Hash[:])
		if err != nil {
			log.Panic(err)
		}*/

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}


// NewBlockchain creates a new Blockchain with genesis Block
func NewRepBlockchain(nodeAdd string) *RepBlockchain {
	dbFile := dbFile+nodeAdd+".db"
	if dbExists(dbFile) == false {
		fmt.Println("No existing blockchain found. Create one first.")
		os.Exit(1)
	}
	var tip [32]byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if b == nil {
			genesis := NewGenesisRepBlock()
			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				log.Panic(err)
			}
			err = b.Put(genesis.Hash[:], genesis.Serialize())
			if err != nil {
				log.Panic(err)
			}
			err = b.Put([]byte("lb"), genesis.Hash[:])
			if err != nil {
				log.Panic(err)
			}
			tip = genesis.Hash
		} else {
			copy(tip[:], b.Get([]byte("lb")))
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	bc := RepBlockchain{tip, db}
	return &bc
}


// CreateRepBlockchain creates a new blockchain DB
func CreateRepBlockchain(nodeAdd string) *RepBlockchain {
	dbFile := dbFile+nodeAdd+".db"
	fmt.Println(dbFile)
	if dbExists(dbFile) {
		fmt.Println("Blockchain already exists.")
		err := os.Remove(dbFile)
		err = os.Remove(dbFile+".lock")
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

	}

	var tip [32]byte
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error {
		CurrentRepBlock.Mu.Lock()
		CurrentRepBlock.Block = NewGenesisRepBlock()
		CurrentRepBlock.Round++
		CurrentRepBlock.Mu.Unlock()

		b, err := tx.CreateBucket([]byte(blocksBucket))
		if err != nil {
			log.Panic(err)
		}

		err = b.Put(CurrentRepBlock.Block.Hash[:], CurrentRepBlock.Block.Serialize())
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("lb"), CurrentRepBlock.Block.Hash[:])
		if err != nil {
			log.Panic(err)
		}
		tip = CurrentRepBlock.Block.Hash

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	bc := RepBlockchain{tip, db}

	return &bc
}


//iterator
func (bc *RepBlockchain) Iterator() *RepBlockchainIterator {
	bci := &RepBlockchainIterator{bc.Tip, bc.Db}

	return bci
}

// Next returns next block starting from the tip
func (i *RepBlockchainIterator) Next() *RepBlock {
	var block *RepBlock

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodedBlock := b.Get(i.currentHash[:])
		block = DeserializeRepBlock(encodedBlock)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	i.currentHash = block.PrevRepBlockHash

	return block
}

// NextSB returns next block starting from the tip
func (i *RepBlockchainIterator) NextFromSB() *SyncBlock {
	var block *SyncBlock

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodedBlock := b.Get(i.currentHash[:])
		block = DeserializeSyncBlock(encodedBlock)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	i.currentHash = block.PrevRepBlockHash

	return block
}

func (i *RepBlockchainIterator) NextToStart() *RepBlock {
	var block *RepBlock
	var flag bool
	flag = false
	for !flag {
		err := i.db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte(blocksBucket))
			encodedBlock := b.Get(i.currentHash[:])
			block = DeserializeRepBlock(encodedBlock)

			return nil
		})
		if err != nil {
			log.Panic(err)
		}
		i.currentHash = block.PrevRepBlockHash
		flag = block.StartBlock
	}
	return block
}


//whether database exisits
func dbExists(dbFile string) bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}