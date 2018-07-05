package main

import (
	"fmt"
	"time"

	"github.com/uchihatmtkinu/RC/Reputation"
	"github.com/uchihatmtkinu/RC/basic"
	"github.com/uchihatmtkinu/RC/gVar"
	"github.com/uchihatmtkinu/RC/rccache"
	"github.com/uchihatmtkinu/RC/shard"
	"github.com/uchihatmtkinu/RC/testforclient/network"
)

func main() {
	ID := 0
	totalepoch := 2
	network.IntilizeProcess(ID)
	fmt.Println("test begin")
	go network.StartServer(ID)
	<-network.IntialReadyCh
	close(network.IntialReadyCh)

	fmt.Println("MyGloablID: ", network.MyGlobalID)
	for k := 1; k <= totalepoch; k++ {
		//test shard
		network.ShardProcess()
		tmptx := make([]basic.Transaction, 4*3)
		for i := 0; i < 4; i++ {
			fmt.Println(shard.GlobalGroupMems[i].RealAccount.Addr, " shard num: ", basic.ShardIndex(shard.GlobalGroupMems[i].RealAccount.AddrReal))
		}
		cnt := 0
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				if i != j {
					tmptx[cnt] = *rccache.GenerateTx(i, j, uint32(cnt+1))
					tmptx[cnt].Print()
					cnt++
				}
			}
		}
		time.Sleep(10 * time.Second)
		tmpBatch := new(basic.TransactionBatch)
		tmpBatch.New(&tmptx)
		data := tmpBatch.Encode()
		network.HandleTotalTx(data)
		for i := 1; i < 4; i++ {
			network.SendTxMessage(shard.GlobalGroupMems[i].Address, "TxM", data)
		}
		if k == 1 {
			go network.TxGeneralLoop()
		}
		//test rep
		network.RepProcess(&shard.GlobalGroupMems)
		Reputation.CurrentRepBlock.Mu.RLock()
		Reputation.CurrentRepBlock.Block.Print()
		Reputation.CurrentRepBlock.Mu.RUnlock()
		/*for i := 0; i < int(gVar.ShardSize); i++ {
			shard.GlobalGroupMems[shard.ShardToGlobal[shard.MyMenShard.Shard][i]].AddRep(int64(shard.ShardToGlobal[shard.MyMenShard.Shard][i]))
		}*/

		time.Sleep(10 * time.Second)

		//test cosi
		if shard.MyMenShard.Role == shard.RoleLeader {
			network.LeaderCosiProcess(&shard.GlobalGroupMems)
		} else {
			network.MemberCosiProcess(&shard.GlobalGroupMems)
		}

		//test sync
		network.SyncProcess(&shard.GlobalGroupMems)
		time.Sleep(10 * time.Second)

		Reputation.CurrentSyncBlock.Mu.RLock()
		Reputation.CurrentSyncBlock.Block.Print()
		Reputation.CurrentSyncBlock.Mu.RUnlock()
		network.CacheDbRef.Mu.Lock()
		fmt.Println("FB from", network.CacheDbRef.ID)
		for i := uint32(0); i < gVar.ShardCnt; i++ {
			network.CacheDbRef.FB[i].Print()
		}
		network.CacheDbRef.Mu.Unlock()

		for i := 0; i < int(gVar.ShardSize*gVar.ShardCnt); i++ {
			shard.GlobalGroupMems[i].Print()
		}

	}

	fmt.Println("All finished")

	time.Sleep(600 * time.Second)

}
