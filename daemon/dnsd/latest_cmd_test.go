package dnsd

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HouzuoGuo/laitos/daemon/common"
	"github.com/HouzuoGuo/laitos/toolbox"
)

func TestLatestCommands(t *testing.T) {
	rec := NewLatestCommands()
	testProcessor := common.GetTestCommandProcessor()

	wg := new(sync.WaitGroup)
	// 3 nested loops and 1 independent command not in a loop
	wg.Add(3*3 + 1)
	// Kick off three concurrent multiple executions of the same command in short succession
	begin := time.Now().Unix()
	var oldResult *toolbox.Result
	for i := 0; i < 3; i++ {
		go func() {
			// Execute the same command in short succession should result in the same output
			result := rec.Execute(testProcessor, common.TestCommandProcessorPIN+".s sleep 1; date")
			oldResult = result // data race is OK
			if result == nil || result.CombinedOutput == "" {
				panic(result)
			}
			for i := 0; i < 3; i++ {
				moreResult := rec.Execute(testProcessor, common.TestCommandProcessorPIN+".s sleep 1; date")
				if moreResult == nil || moreResult.CombinedOutput != result.CombinedOutput {
					panic(moreResult)
				}
				wg.Done()
				time.Sleep(1 * time.Second)
			}
		}()
	}
	go func() {
		result := rec.Execute(testProcessor, common.TestCommandProcessorPIN+".s sleep 1; echo hi")
		if result == nil || strings.TrimSpace(result.CombinedOutput) != "hi" {
			panic(result)
		}
		wg.Done()
	}()
	wg.Wait()
	if len(rec.latestResult) != 2 { // echo hi and date
		t.Fatal(rec.latestResult)
	}
	// Make sure that the commands are indeed executed in parallel
	if time.Now().Unix()-begin > 4 {
		t.Fatal("did not execute in parallel")
	}

	// Wait until TTL expires, date command must not return the same content.
	time.Sleep((TextCommandReplyTTL + 1) * time.Second)
	result := rec.Execute(testProcessor, common.TestCommandProcessorPIN+".s sleep 1; date")
	if result == nil || result.CombinedOutput == "" || result.CombinedOutput == oldResult.CombinedOutput {
		t.Fatal(result)
	}
	if len(rec.latestResult) != 1 { // date by itself
		t.Fatal(rec.latestResult)
	}
}
