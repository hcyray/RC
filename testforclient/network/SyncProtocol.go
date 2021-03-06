package network

import (
	"bytes"
	"encoding/gob"
	"github.com/uchihatmtkinu/PriRC/snark"
	"log"
	"sync"
	"time"

	"fmt"
	"math/big"
	"math/rand"

	"github.com/uchihatmtkinu/PriRC/Reputation"
	"github.com/uchihatmtkinu/PriRC/base58"
	"github.com/uchihatmtkinu/PriRC/basic"
	"github.com/uchihatmtkinu/PriRC/gVar"
	"github.com/uchihatmtkinu/PriRC/shard"
)

//sbrxCounter count for the block receive
//var sbrxCounter safeCounter

//aski ask user i for sync, -1 means done, otherwise ask i+1
var aski []int

//SyncProcess processing sync after one epoch
func SyncProcess(ms *[]shard.MemShard) {

	CurrentEpoch++
	fmt.Println("Sync Began")
	startDone = false
	//waitgroup for all goroutines done
	var wg sync.WaitGroup
	aski = make([]int, int(gVar.ShardCnt))
	rand.Seed(int64(shard.MyMenShard.Shard*3000+shard.MyMenShard.InShardId) + time.Now().UTC().UnixNano())
	//intilizeMaskBit(&syncmask, int((gVar.ShardCnt+7)>>3),false)
	for i := 0; i < int(gVar.ShardCnt); i++ {
		syncSBCh[i] = make(chan syncSBInfo, 10)
		syncTBCh[i] = make(chan syncTBInfo, 10)
		syncNotReadyCh[i] = make(chan bool, 10)
		aski[i] = rand.Intn(int(gVar.ShardSize))
	}
	SyncFlag = true

	for i := 0; i < int(gVar.ShardCnt); i++ {
		//if !maskBit(i, &syncmask) {
		if i != shard.MyMenShard.Shard {
			wg.Add(1)
			go SendSyncMessage((*ms)[shard.ShardToGlobal[i][aski[i]]].Address, "requestSync", syncRequestInfo{ID: MyGlobalID, Epoch: CurrentEpoch})
			go ReceiveSyncProcess(i, &wg, ms)
		}
		//}
	}
	wg.Wait()
	SyncFlag = false
	for i := 0; i < int(gVar.ShardCnt); i++ {
		close(syncSBCh[i])
		close(syncTBCh[i])
		close(syncNotReadyCh[i])
	}
	if gVar.RandomAttack {
		RandomAttack(ms)
	}
	fmt.Println("Sync Finished")
	shard.TotalRep = 0
	for i := 0; i < int(gVar.ShardSize*gVar.ShardCnt); i++ {
		snark.BabyJubJubCurve.VerifyPedersenCommit((*ms)[i].CalTotalRep(), int32((*ms)[i].AttackID+1), &(*ms)[i].RepComm)
		shard.TotalRep += (*ms)[i].CalTotalRep()
	}
	fmt.Println("TotalRep:", shard.TotalRep)
}

func RandomAttack(ms *[]shard.MemShard) {
	(*ms)[shard.ShardToGlobal[shard.MyMenShard.Shard][0]].Bandwidth, (*ms)[LeaderBandID].Bandwidth =
		(*ms)[LeaderBandID].Bandwidth, (*ms)[shard.ShardToGlobal[shard.MyMenShard.Shard][0]].Bandwidth
	n := int(gVar.ShardCnt * gVar.ShardSize)
	oldRep := make([]int64, n)
	oldSumRep := make([]int64, n)
	oldTotalRep := make([][]int64, n)
	oldBand := make([]int, n)
	oldRepComm := make([]snark.PedersenCommitment, n)
	oldID := make([]int, n)
	old := new(big.Int)
	for i := 0; i < n; i++ {
		oldRep[i] = (*ms)[i].Rep
		oldSumRep[i] = (*ms)[i].CalTotalRep()
		oldBand[i] = (*ms)[i].Bandwidth
		oldRepComm[i].Init()
		oldRepComm[i].Comm_x.Add((*ms)[i].RepComm.Comm_x, old)
		oldRepComm[i].Comm_y.Add((*ms)[i].RepComm.Comm_y, old)
		oldID[i] = (*ms)[i].AttackID
		oldTotalRep[i] = make([]int64, len((*ms)[i].TotalRep))
		copy(oldTotalRep[i], (*ms)[i].TotalRep)
	}
	rand.Seed(int64(CurrentEpoch + 10))
	ri := rand.Perm(int(gVar.ShardSize * gVar.ShardCnt))
	for i := 0; i < n; i++ {
		oldRep[i], oldRep[ri[i]] = oldRep[ri[i]], oldRep[i]
		oldSumRep[i], oldSumRep[ri[i]] = oldSumRep[ri[i]], oldSumRep[i]
		oldBand[i], oldBand[ri[i]] = oldBand[ri[i]], oldBand[i]
		oldRepComm[i], oldRepComm[ri[i]] = oldRepComm[ri[i]], oldRepComm[i]
		oldID[i], oldID[ri[i]] = oldID[ri[i]], oldID[i]
		oldTotalRep[i], oldTotalRep[ri[i]] = oldTotalRep[ri[i]], oldTotalRep[i]
	}
	for i := 0; i < n; i++ {
		(*ms)[i].Rep = oldRep[i]
		(*ms)[i].Bandwidth = oldBand[i]
		copy((*ms)[i].TotalRep, oldTotalRep[i])
		(*ms)[i].SetPriRepPC(oldRepComm[i])
		(*ms)[i].AttackID = oldID[i]
	}
}

//ReceiveSyncProcess listen to the block from shard k
func ReceiveSyncProcess(k int, wg *sync.WaitGroup, ms *[]shard.MemShard) {
	fmt.Println("wait for shard", k, " from user", shard.ShardToGlobal[k][aski[k]])
	defer wg.Done()

	//syncblock flag
	sbrxflag := true
	//txblock flag

	tbrxflag := true
	//txBlock Transaction block

	var txBlockMessage syncTBInfo
	//syncBlock SyncBlock
	var syncBlockMessage syncSBInfo
	for sbrxflag || tbrxflag {
		select {
		case syncBlockMessage = <-syncSBCh[k]:
			{
				if syncBlockMessage.Block.VerifyCoSignature(ms) {
					sbrxflag = false
					fmt.Println("Get cosi from", k)
				} else {
					//aski[k] = (aski[k] + 1) % int(gVar.ShardSize)
					fmt.Println("Verifyied cosi falied")
					sbrxflag = false

					//tbrxflag = true
					//go SendSyncMessage((*ms)[shard.ShardToGlobal[k][aski[k]]].Address, "requestSync", syncRequestInfo{MyGlobalID, CurrentEpoch})
				}
			}

		case txBlockMessage = <-syncTBCh[k]:
			tmpIndex := 0
			if gVar.ExperimentBadLevel != 0 {
				for shard.ShardToGlobal[k][tmpIndex] < int(gVar.ShardCnt*gVar.ShardSize/3) {
					tmpIndex++
				}
			}
			ok, err := txBlockMessage.Block.Verify(&(*ms)[shard.ShardToGlobal[k][tmpIndex]].RealAccount.Puk)
			if ok {
				tbrxflag = false
				tmpBlock := txBlockMessage.Block
				CacheDbRef.FB[k] = &tmpBlock
				fmt.Println("Get FB from", k)
			} else {
				fmt.Println("FinalTxBlock verify failed: ", err)
			}
		case <-syncNotReadyCh[k]:
			fmt.Println(time.Now(), "sleep for not ready")
			time.Sleep(timeSyncNotReadySleep)
			go SendSyncMessage((*ms)[shard.ShardToGlobal[k][aski[k]]].Address, "requestSync", syncRequestInfo{ID: MyGlobalID, Epoch: CurrentEpoch})
		case <-time.After(timeoutSync):
			{
				aski[k] = (aski[k] + 1) % int(gVar.ShardSize)
				fmt.Println(time.Now(), "wait for shard", k, " from user", shard.ShardToGlobal[k][aski[k]])
				go SendSyncMessage((*ms)[shard.ShardToGlobal[k][aski[k]]].Address, "requestSync", syncRequestInfo{ID: MyGlobalID, Epoch: CurrentEpoch})
			}
		}
	}
	if !sbrxflag && !tbrxflag {
		//add transaction block

		CacheDbRef.Mu.Lock()
		CacheDbRef.GetFinalTxBlock(&txBlockMessage.Block)
		CacheDbRef.Mu.Unlock()
		//update reputation of members
		syncBlockMessage.Block.UpdateTotalRepInMS(ms)
		//add sync Block
		Reputation.MyRepBlockChain.AddSyncBlockFromOtherShards(&syncBlockMessage.Block, k)
	}
	fmt.Println("received sync from shard:", k)
}

//SendSyncMessage send cosi message
func SendSyncMessage(addr string, command string, message interface{}) {
	payload := gobEncode(message)
	request := append(commandToBytes(command), payload...)
	sendData(addr, request)
}

//HandleSyncNotReady rx challenge
func HandleSyncNotReady(request []byte) {
	var buff bytes.Buffer
	var payload syncNotReadyInfo
	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	syncNotReadyCh[shard.GlobalGroupMems[payload.ID].Shard] <- true
}

//HandleRequestSync handles the sync request
func HandleRequestSync(request []byte) {
	var buff bytes.Buffer
	var payload syncRequestInfo

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}
	addr := shard.GlobalGroupMems[payload.ID].Address
	fmt.Println("Recived request sync from ", payload.ID)
	Reputation.CurrentSyncBlock.Mu.RLock()
	defer Reputation.CurrentSyncBlock.Mu.RUnlock()
	if payload.Epoch > Reputation.CurrentSyncBlock.Epoch {
		SendSyncMessage(addr, "syncNReady", syncNotReadyInfo{MyGlobalID, Reputation.CurrentSyncBlock.Epoch})
		return
	}
	if payload.Epoch == Reputation.CurrentSyncBlock.Epoch {

		tmp := syncTBInfo{MyGlobalID, *(CacheDbRef.FB[CacheDbRef.HistoryShard[payload.Epoch]])}
		sendTxMessage(addr, "syncTB", tmp.Encode())
		SendSyncMessage(addr, "syncSB", syncSBInfo{MyGlobalID, *Reputation.CurrentSyncBlock.Block})
		return
	}

}

//HandleSyncSBMessage rx challenge
func HandleSyncSBMessage(request []byte) {
	var buff bytes.Buffer
	var payload syncSBInfo

	buff.Write(request)
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	fmt.Println("SyncSBdata:", payload.ID, "Hash:", base58.Encode(payload.Block.Hash[:]))
	if err != nil {
		log.Panic(err)
	}
	syncSBCh[shard.GlobalGroupMems[payload.ID].Shard] <- payload
}

//HandleSyncTBMessage decodes the txblock
func HandleSyncTBMessage(request []byte) {
	var payload syncTBInfo
	tmpdata := make([]byte, len(request))
	copy(tmpdata, request)
	err := payload.Decode(&tmpdata)

	fmt.Println("SyncTBdata:", payload.ID, "Hash:", base58.Encode(payload.Block.HashID[:]))
	//payload.Block.Print()
	if err != nil {
		log.Panic(err)
	}
	syncTBCh[shard.GlobalGroupMems[payload.ID].Shard] <- payload

}

//Encode is the encode
func (s *syncTBInfo) Encode() []byte {
	var tmp []byte
	basic.Encode(&tmp, uint32(s.ID))
	tmp = append(tmp, s.Block.Serial()...)
	fmt.Println("syncTBInfo length: ", len(tmp))
	return tmp
}

//Decode is the decode
func (s *syncTBInfo) Decode(buf *[]byte) error {
	var tmp uint32
	basic.Decode(buf, &tmp)
	s.ID = int(tmp)
	err := s.Block.Decode(buf, 1)
	return err
}
