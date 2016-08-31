package main

import (
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"github.com/streamrail/concurrent-map"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"time"
	"io/ioutil"
	"github.com/satori/go.uuid"
)

// TODO: note can also have a volume
type Note struct {
	Id       string  `json:"id"`
	Start    float32 `json:"start"`
	End      float32 `json:"end"`
	Note     int     `json:"note"`
	Velocity int     `json:"velocity"`
}

type Bar struct {
	Notes []Note `json:"notes"`
	Id string `json:"id"`
}

type Vote struct {
	Key   string
	Value int
}

type VoteList []Vote

func (v VoteList) Len() int {
	return len(v)
}

func (v VoteList) Less(i, j int) bool {
	return v[i].Value < v[j].Value
}

func (v VoteList) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

func (v VoteList) Get(i int) Vote {
	return v[i]
}

var (
	votes          = cmap.New()
	currentBar     = Bar{}
	numBars        = 10
	numNotes       = 10
	randomError = 4
)

func RandBool() bool {
	return rand.Intn(1) == 1
}

func RandError() int {
	if RandBool(){
		return rand.Intn(randomError)
	}
	return -rand.Intn(randomError)
}

func GenerateNote() Note {
	n := Note{}
	n.Start = 0
	n.End = rand.Float32()
	n.Note = rand.Intn(127)
	n.Velocity = rand.Intn(127)
	n.Id = uuid.NewV4().String()
	return n
}

func GenerateBars() []Bar {
	bars := make([]Bar, numBars)
	for i := 0; i < numBars; i++ {
		m := rand.Intn(numNotes) + numNotes
		b := Bar{}
		b.Notes = make([]Note, m)
		for i := 0; i < m; i++ {
			b.Notes[i] = GenerateNote()
		}
		b.Id = uuid.NewV4().String()
		votes.Set(b.Id, 0)
		bars[i] = b
	}
	return bars
}

func ReproduceNotes(mother, father Note) Note {
	child := Note{}
	child.Start = 0

	if RandBool() {
		if RandBool() {
			child.End = mother.End
		} else {
			child.End = father.End
		}
	} else {
		child.End = rand.Float32()
	}

	if RandBool() {
		child.Note = mother.Note + RandError()
	} else {
		child.Note = father.Note + RandError()
	}

	if RandBool() {
		child.Velocity = mother.Velocity + RandError()
	} else {
		child.Velocity = father.Velocity + RandError()
	}

	if child.Note > 127 {
		child.Note = 127
	}

	if child.Note < 22 {
		child.Note = 22
	}

	if child.Velocity > 127 {
		child.Velocity = 127
	}

	if child.Velocity < 70 {
		child.Velocity = 70 + randomError
	}

	child.Id = uuid.NewV4().String()
	return child
}

func BreedBars(bars []Bar, v map[string]int) []Bar {
	log.Println("starting to breed new notes")
	// new slice of bars
	newBars := make([]Bar, len(bars))
	parents := []string{}

	totalVotes := 0
	for _, e := range v {
		totalVotes += e
	}

	if totalVotes > 0 {
		// inverted votes index
		invertedIndex := make(VoteList, len(v))
		i := 0
		avgVotes := 0
		for k, e := range v {
			invertedIndex[i] = Vote{Key: k, Value: e}
			avgVotes += e
			i++
		}
		avgVotes = avgVotes / len(v)

		sort.Sort(sort.Reverse(invertedIndex))

		for i := range invertedIndex {
			vote := invertedIndex.Get(i)
			if vote.Value > avgVotes {
				parents = append(parents, vote.Key)
			}
		}
	} else {
		for i := range bars {
			parents = append(parents, bars[i].Id)
		}
	}


	barsMap := make(map[string]Bar, len(bars))
	for i := range bars {
		barsMap[bars[i].Id] = bars[i]
	}
	parentsLen := len(parents)
	parentBars := make(map[string]Bar, parentsLen)
	for _, parent := range parents {
		if e, ok := barsMap[parent]; ok {
			parentBars[parent] = e
		}
	}

	for i := 0; i < numBars; i++ {
		m := rand.Intn(numNotes) + numNotes
		b := Bar{}
		b.Notes = make([]Note, m)

		bar := parentBars[parents[rand.Intn(parentsLen)]]

		for j := 0; j < m; j++ {
			mother := bar.Notes[rand.Intn(len(bar.Notes))]
			father := bar.Notes[rand.Intn(len(bar.Notes))]
			b.Notes[j] = ReproduceNotes(mother, father)
		}
		b.Id = uuid.NewV4().String()
		votes.Set(b.Id, 0)
		newBars[i] = b
	}

	return newBars
}

func Conduct() {
	log.Println("Starting Conductor...")
	bars := GenerateBars()
	c := 0
	for true {
		//log.Println(c)
		// non-blocking send signal to channel
		currentBar = bars[c]
		bar := bars[c]

		log.Println("current bar", bar.Id)
		// play the notes on the server
		for i := 0; i < len(bar.Notes); i++ {
			note := bar.Notes[i]
			time.Sleep(time.Duration((1 - note.End) * 1000 * 1000 * 1000))
		}

		c++

		if c >= numBars {
			// copy the votes and notes before deleting
			v := make(map[string]int, votes.Count())
			for i, e := range votes.Items() {
				v[i] = e.(int)
			}

			// clear the votes from the cache
			for i := 0; i < numBars; i++ {
				for j := 0; j < len(bars); j++ {
					votes.Remove(bars[i].Id)
				}
			}

			// reset the counter
			c = 0

			// run genetic algorithm
			bars = BreedBars(bars, v)
		}
	}
}

func Index(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	file, err := ioutil.ReadFile("index.html")
	if err != nil {
		panic(err)
	}
	w.Header().Add("Content-Type", "text/html")
	w.Write(file)
}

func VoteBar(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	barId := p.ByName("barId")
	if currentVotes, ok := votes.Get(barId); ok {
		votes.Set(barId, currentVotes.(int) + 1)
	} else {
		log.Println("something went wrong")
	}
}

func CurrentBar(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	data, err := json.Marshal(currentBar)
	if err != nil {
		fmt.Errorf("%v", err)
	}
	w.Write(data)
}

func main() {
	rand.Seed(time.Now().Unix())
	go Conduct()

	router := httprouter.New()

	router.GET("/", Index)
	router.GET("/bar", CurrentBar)
	router.GET("/vote/:barId", VoteBar)

	router.ServeFiles("/static/*filepath", http.Dir("static"))

	log.Println("Started Server!")

	log.Fatal(http.ListenAndServe(":8080", router))
}
