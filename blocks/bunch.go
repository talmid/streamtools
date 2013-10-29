package blocks

import (
	"container/heap"
	"encoding/json"
	"log"
	"strings"
	"time"
)

func Bunch(b *Block) {

	type bunchRule struct {
		Branch    string
		EmitAfter int
	}

	rule := &bunchRule{}

	unmarshal(<-b.Routes["set_rule"], &rule)
	branchString := rule.Branch
	//log.Println("grouping by", branchString)
	afterSeconds := rule.EmitAfter
	//log.Println("emitting after", afterSeconds, "seconds")

	after := time.Duration(afterSeconds) * time.Second
	branch := strings.Split(branchString, ".")

	bunches := make(map[string][]*BMsg)
	waitTimer := time.NewTimer(100 * time.Millisecond)
	pq := &PriorityQueue{}
	heap.Init(pq)

	for {
		select {
		case msg := <-b.AddChan:
			updateOutChans(msg, b)
		case <-b.QuitChan:
			quit(b)
			return
		case msg := <-b.InChan:
			id, err := Get(msg, branch...)
			idStr, ok := id.(string)
			if !ok {
				log.Fatal("type assertion failed")
			}
			if err != nil {
				log.Fatal(err.Error())
			}
			if len(bunches[idStr]) > 0 {
				bunches[idStr] = append(bunches[idStr], msg)
			} else {
				bunches[idStr] = []*BMsg{msg}
			}

			var val interface{}
			if err != nil {
				log.Fatal(err.Error())
			}
			Set(val, "id", idStr)
			Set(val, "length", len(bunches[idStr]))

			blob, err := json.Marshal(val)
			if err != nil {
				log.Fatal(err.Error())
			}

			queueMessage := &PQMessage{
				val: &blob,
				t:   time.Now(),
			}
			heap.Push(pq, queueMessage)
		case <-waitTimer.C:
		}
		for {
			pqMsg, diff := pq.PeekAndShift(time.Now(), after)
			if pqMsg == nil {
				// either the queue is empty, or it's not time to emit
				waitTimer.Reset(diff)
				break
			}
			v := pqMsg.(*PQMessage).val
			queueMessage, err := simplejson.NewJson(*v)
			if err != nil {
				log.Fatal(err.Error())
			}
			l, err := queueMessage.Get("length").Int()
			if err != nil {
				log.Fatal(err.Error())
			}
			id, err := queueMessage.Get("id").String()
			if err != nil {
				log.Fatal(err.Error())
			}
			if l == len(bunches[id]) {
				// we've not seen anything since putting this message in the queue
				outMsg, err := simplejson.NewJson([]byte("{}"))
				if err != nil {
					log.Fatal(err.Error())
				}
				outMsg.Set("bunch", bunches[id])
				broadcast(b.OutChans, outMsg)
				delete(bunches, id)
			}
		}
	}
}
