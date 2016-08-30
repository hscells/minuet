package main

import (
	"encoding/json"
	"fmt"
	"github.com/cnf/structhash"
	"github.com/julienschmidt/httprouter"
	"github.com/streamrail/concurrent-map"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"time"
	"io/ioutil"
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
	notes          = cmap.New()
	// TODO: votes should be for a bar, not a note!
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
	n.Note = rand.Intn(255)
	n.Velocity = rand.Intn(255)

	// side effect: adds the note to the list of notes
	h, err := structhash.Hash(n, 1)
	if err != nil {
		log.Panic(err)
	}

	n.Id = h

	notes.Set(h, n)
	votes.Set(h, rand.Intn(50))
	return n
}

func GenerateBars() []Bar {
	bars := make([]Bar, numBars)
	for i := 0; i < numBars; i++ {
		m := rand.Intn(numNotes)
		b := Bar{}
		b.Notes = make([]Note, m)
		for i := 0; i < m; i++ {
			b.Notes[i] = GenerateNote()
		}
		h, err := structhash.Hash(b, 1)
		if err != nil {
			log.Panic(err)
		}
		b.Id = h

		bars[i] = b

	}
	return bars
}

func ReproduceNotes(parent1, parent2 string, n map[string]Note) Note {
	note1 := n[parent1]
	note2 := n[parent2]
	child := Note{}

	child.Start = 0

	if RandBool() {
		if RandBool() {
			child.End = note1.End
		} else {
			child.End = note2.End
		}
	} else {
		child.End = rand.Float32()
	}

	if RandBool() {
		child.Note = note1.Note + RandError()
	} else {
		child.Note = note2.Note + RandError()
	}

	if RandBool() {
		child.Velocity = note1.Velocity + RandError()
	} else {
		child.Velocity = note2.Velocity + RandError()
	}

	// side effect: adds the note to the list of notes
	h, err := structhash.Hash(child, 1)
	if err != nil {
		log.Panic(err)
	}

	if child.Note > 106 {
		child.Note = 106
	}

	if child.Note < 22 {
		child.Note = 22
	}

	if child.Velocity > 255 {
		child.Velocity = 255
	}

	if child.Velocity < 70 {
		child.Velocity = 70 + randomError
	}

	child.Id = h
	notes.Set(h, child)
	votes.Set(h, rand.Intn(50))
	return child
}

func BreedBars(bars []Bar, v map[string]int, n map[string]Note) []Bar {
	log.Println("starting to breed new notes")

	// new slice of bars
	newBars := make([]Bar, len(bars))

	// inverted votes index
	invertedIndex := make(VoteList, len(v))
	parents := []string{}
	i := 0
	avgVotes := 0
	for k, e := range v {
		invertedIndex[i] = Vote{Key: k, Value: e}
		avgVotes += e
		i++
	}
	avgVotes = avgVotes / len(v)

	sort.Sort(sort.Reverse(invertedIndex))
	//log.Println("index: ", invertedIndex)

	for i := range invertedIndex {
		vote := invertedIndex.Get(i)
		if vote.Value < avgVotes {
			parents = append(parents, vote.Key)
		}
	}

	//log.Println("parents: ", parents)
	parentsLen := len(parents)

	for i := 0; i < numBars; i++ {
		m := rand.Intn(numNotes)
		b := Bar{}
		b.Notes = make([]Note, m)
		for i := 0; i < m; i++ {
			b.Notes[i] = ReproduceNotes(parents[rand.Intn(parentsLen)], parents[rand.Intn(parentsLen)], n)
		}
		h, err := structhash.Hash(b, 1)
		if err != nil {
			log.Panic(err)
		}
		b.Id = h
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

		log.Println("current bar", bar)
		// play the notes on the server
		for i := 0; i < len(bar.Notes); i++ {
			note := bar.Notes[i]
			time.Sleep(time.Duration((1 - note.End) * 1000 * 1000 * 1000))
		}

		c++

		if c == numBars {
			// copy the votes and notes before deleting
			v := make(map[string]int, votes.Count())
			for i, e := range votes.Items() {
				v[i] = e.(int)
			}

			n := make(map[string]Note, notes.Count())
			for i, e := range notes.Items() {
				n[i] = e.(Note)
			}

			// clear the notes from the cache
			for i := 0; i < numBars; i++ {
				for j := 0; j < len(bars[i].Notes); j++ {
					notes.Remove(bars[i].Notes[j].Id)
					votes.Remove(bars[i].Notes[j].Id)
				}
			}

			// reset the counter
			c = 0

			// run genetic algorithm
			bars = BreedBars(bars, v, n)
			//log.Println("new bars:", bars)
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

func VoteOnNote(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	p.ByName("noteId")
	// TODO: somehow need to vote up the current bar.
	// this needs to be renamed to VoteOnBar
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
	router.GET("/vote/:nodeId", VoteOnNote)

	router.ServeFiles("/static/*filepath", http.Dir("static"))

	log.Println("Started Server!")

	log.Fatal(http.ListenAndServe(":8080", router))
}
