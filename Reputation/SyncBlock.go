package Reputation

import (
	"time"
	"bytes"
	"encoding/gob"
	"log"
	"crypto/sha256"
	"github.com/uchihatmtkinu/RC/gVar"
	"github.com/uchihatmtkinu/RC/shard"
	"github.com/uchihatmtkinu/RC/Reputation/cosi"
	"github.com/uchihatmtkinu/RC/ed25519"
	"fmt"
)

//SyncBlock syncblock
type SyncBlock struct {
	Timestamp     	 	int64
	PrevRepBlockHash 	[32]byte
	PrevSyncBlockHash 	[][32]byte
	IDlist				[]int
	TotalRep			[][] int64//Total reputation over epoch, [i][j] i-th user, j-th epoch
	CoSignature			[]byte
	Hash          	 	[32]byte
}

// NewSynBlock new sync block
func NewSynBlock(ms *[]shard.MemShard, prevSyncBlockHash[][32]byte, prevRepBlockHash [32]byte, coSignature []byte) *SyncBlock{
	var item *shard.MemShard
	var repList [][]int64
	var idList	[]int
	tmpprevSyncBlockHash:=make([][32]byte,len(prevSyncBlockHash))
	copy(tmpprevSyncBlockHash,prevSyncBlockHash)
	tmpcoSignature:=make([]byte,len(coSignature))
	copy(tmpcoSignature,coSignature)

	//mask := coSignature[64:]
	//repList = make([][gVar.SlidingWindows]int64, 0)

	for i := 0; i < int(gVar.ShardSize); i++{
		item = &(*ms)[shard.ShardToGlobal[shard.MyMenShard.Shard][i]]
		//need to consider if a node fail to sign the syncBlock but it is a good node indeed
		item.SetTotalRep(item.Rep)
		idList = append(idList, shard.ShardToGlobal[shard.MyMenShard.Shard][i])
		repList = append(repList, item.TotalRep)
	}

	block := &SyncBlock{time.Now().Unix(), prevRepBlockHash,tmpprevSyncBlockHash, idList, repList,tmpcoSignature, [32]byte{}}
	block.Hash = sha256.Sum256(block.prepareData())
	return block
}

// prepareData prepare []byte data
func (b *SyncBlock) prepareData() []byte {
	data := bytes.Join(
		[][]byte{
			b.PrevRepBlockHash[:],
			b.HashPrevSyncBlock(),
			b.HashIDList(),
			b.HashTotalRep(),
			b.CoSignature,
			//IntToHex(b.Timestamp),
		},
		[]byte{},
	)

	return data
}


// HashRep returns a hash of the TotalRepTransactions in the block
func (b *SyncBlock) HashTotalRep() []byte {
	var txHashes []byte
	var txHash [32]byte
	for i := range b.TotalRep {
		for _,item := range b.TotalRep[i] {
			txHashes = append(txHashes, IntToHex(item)[:]...)
		}

	}
	txHash = sha256.Sum256(txHashes)
	return txHash[:]
}


// HashIDList returns a hash of the IDList in the block
func (b *SyncBlock) HashIDList() []byte {
	var txHashes []byte
	var txHash [32]byte
	for _,item := range b.IDlist {
		txHashes = append(txHashes, IntToHex(int64(item))[:]...)
	}
	txHash = sha256.Sum256(txHashes)
	return txHash[:]
}

// HashPrevSyncBlock returns a hash of the previous sync block hash
func (b *SyncBlock) HashPrevSyncBlock() []byte {
	var txHashes []byte
	var txHash [32]byte
	for _,item := range b.PrevSyncBlockHash {
		txHashes = append(txHashes, item[:]...)
	}
	txHash = sha256.Sum256(txHashes)
	return txHash[:]
}


// VerifyCosign verify CoSignature, k-th shard
func (b *SyncBlock) VerifyCoSignature(ms *[]shard.MemShard) bool {
	// count whether half of the user sign the block
	count := 0
	mask := b.CoSignature[64:]
	for i := 0; i < int(gVar.ShardSize); i++ {
		if maskBit(i, &mask) == cosi.Enabled{
			count++
		}
	}
	if count < int(gVar.ShardSize/2) {
		return false
	}
	//verify signature
	var	pubKeys		[]ed25519.PublicKey
	var it *shard.MemShard
	sbMessage := b.PrevRepBlockHash[:]
	pubKeys = make([]ed25519.PublicKey, int(gVar.ShardSize))
	for i:=0; i < int(gVar.ShardSize); i++ {
		it = &(*ms)[b.IDlist[i]]
		pubKeys[i] = it.CosiPub
	}
	valid := cosi.Verify(pubKeys, nil, sbMessage, b.CoSignature)
	return valid
}

// UpdateRepToTotalRepInMS update the rep to total rep in memshards
func (b *SyncBlock) UpdateTotalRepInMS(ms *[]shard.MemShard) {
	var item *shard.MemShard
	for i,id := range b.IDlist{
		item = &(*ms)[id]
		item.CopyTotalRepFromSB(b.TotalRep[i])
	}
}

//Print print sync block
func (b *SyncBlock) Print() {
	fmt.Println("SyncBlock:")
	fmt.Println("PrevSyncBlockHash:", b.PrevSyncBlockHash)
	fmt.Println("RepTransactions:")
	for i,item := range b.IDlist{
		fmt.Print("	GlobalID:", item)
		fmt.Println("		TotalRep",b.TotalRep[i])
	}

	fmt.Println("CoSignature:", b.CoSignature)
	fmt.Println("PrevRepBlockHash:", b.PrevRepBlockHash)
	fmt.Println("Hash:", b.Hash)
}


// Serialize encode block
func (b *SyncBlock) Serialize() []byte {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return result.Bytes()
}


// DeserializeSyncBlock decode Syncblock
func DeserializeSyncBlock(d []byte) *SyncBlock {
	var block SyncBlock
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}


