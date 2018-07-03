package rccache

import (
	"crypto/x509"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/uchihatmtkinu/RC/basic"

	"github.com/uchihatmtkinu/RC/base58"
	"github.com/uchihatmtkinu/RC/cryptonew"
	"github.com/uchihatmtkinu/RC/gVar"

	"github.com/uchihatmtkinu/RC/shard"

	"github.com/uchihatmtkinu/RC/account"
)

func TestGeneratePriKey(t *testing.T) {
	file, _ := os.Create("PriKeys.txt")
	for i := 0; i < 5; i++ {
		var tmp account.RcAcc
		tmp.New(strconv.Itoa(i))
		tmp.NewCosi()
		fmt.Println(tmp.Pri)

		tmpHash, _ := x509.MarshalECPrivateKey(&tmp.Pri)
		//fmt.Println(len(tmpHash))
		file.Write(tmpHash)
		file.Write(tmp.CosiPri)
	}
	file.Close()
	//t.Error("No file")
}

func GenerateTx(x int, y int, z uint32) *basic.Transaction {
	var tmp basic.Transaction
	tmp.New(0)
	var b basic.OutType
	b.Address = shard.GlobalGroupMems[y].RealAccount.AddrReal
	b.Value = z
	var a basic.InType
	a.Init()
	a.PrevTx = shard.GlobalGroupMems[x].RealAccount.AddrReal
	a.Index = z
	tmp.AddOut(b)
	tmp.AddIn(a)
	tmp.Hash = tmp.HashTx()
	tmp.SignTx(0, &shard.GlobalGroupMems[x].RealAccount.Pri)
	return &tmp
}

func TestOutToData(t *testing.T) {
	numCnt := 4
	acc := make([]account.RcAcc, numCnt)
	dbs := make([]DbRef, numCnt)
	shard.GlobalGroupMems = make([]shard.MemShard, numCnt)
	file, ok := os.Open("PriKeys.txt")
	if ok != nil {
		t.Error("No file")
	}
	accWallet := make([]basic.AccCache, numCnt)
	for i := 0; i < numCnt; i++ {
		acc[i].New(strconv.Itoa(i))
		acc[i].NewCosi()
		tmp1 := make([]byte, 121)
		tmp2 := make([]byte, 64)
		file.Read(tmp1)
		file.Read(tmp2)
		xxx, _ := x509.ParseECPrivateKey(tmp1)
		acc[i].Pri = *xxx
		acc[i].Puk = acc[i].Pri.PublicKey
		acc[i].CosiPri = tmp2
		acc[i].CosiPuk = tmp2[32:]

		acc[i].AddrReal = cryptonew.AddressGenerate(&acc[i].Pri)
		acc[i].Addr = base58.Encode(acc[i].AddrReal[:])
		accWallet[i].ID = acc[i].AddrReal
		accWallet[i].Value = 100
		//tmp, _ := x509.MarshalECPrivateKey(&acc[i].Pri)
		shard.GlobalGroupMems[i].NewMemShard(&acc[i], "123")
		dbs[i].New(uint32(i), acc[i].Pri)
	}
	t.Error("Check1")

	shard.ShardToGlobal = make([][]int, gVar.ShardCnt)
	for i := uint32(0); i < gVar.ShardCnt; i++ {
		shard.ShardToGlobal[i] = make([]int, gVar.ShardSize)
		for j := uint32(0); j < gVar.ShardSize; j++ {
			shard.ShardToGlobal[i][j] = int(i*2 + j)
			dbs[i*2+j].ShardNum = i
			shard.GlobalGroupMems[i*2+j].Shard = int(i)
		}
	}
	for i := 0; i < numCnt; i++ {
		for j := 0; j < numCnt; j++ {
			dbs[i].DB.AddAccount(&accWallet[j])
		}
		dbs[i].DB.UploadAcc(dbs[i].ShardNum)
		//dbs[i].DB.ShowAccount()
	}
	tmp := GenerateTx(0, 1, 10)
	tmp.Print()
	dbs[2].MakeTXList(tmp)
	dbs[2].BuildTDS()
	dbs[3].GetTx(tmp)
	dbs[2].TLS[0].Print()
	dbs[3].ProcessTL(&dbs[2].TLS[0])
	dbs[1].GetTx(tmp)
	dbs[0].GetTx(tmp)
	dbs[2].NewTxList()
	dbs[3].TLNow.Print()
	dbs[2].UpdateTXCache(dbs[3].TLNow)
	dbs[2].TDSCache[0][0].Print()
	dbs[2].ProcessTDS(&dbs[2].TDSCache[0][1])
	dbs[0].ProcessTDS(&dbs[2].TDSCache[0][0])
	fmt.Println(len(dbs[2].Ready))
	dbs[1].GetTDS(&dbs[2].TDSCache[0][0])
	fmt.Println(len(dbs[0].Ready))
	file.Close()
	t.Error("Check")
}
