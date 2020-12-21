package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"index/suffixarray"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

func main() {

	plays := []string{
		"THE SONNETS",
		"ALL’S WELL THAT ENDS WELL",

		"ANTONY AND CLEOPATRA",

		"AS YOU LIKE IT",

		"THE COMEDY OF ERRORS",

		"THE TRAGEDY OF CORIOLANUS",

		"CYMBELINE",

		"THE TRAGEDY OF HAMLET, PRINCE OF DENMARK",

		"THE FIRST PART OF KING HENRY THE FOURTH",

		"THE SECOND PART OF KING HENRY THE FOURTH",

		"THE LIFE OF KING HENRY THE FIFTH",

		"THE FIRST PART OF HENRY THE SIXTH",

		"THE SECOND PART OF KING HENRY THE SIXTH",

		"THE THIRD PART OF KING HENRY THE SIXTH",

		"KING HENRY THE EIGHTH",

		"KING JOHN",

		"THE TRAGEDY OF JULIUS CAESAR",

		"THE TRAGEDY OF KING LEAR",

		"LOVE’S LABOUR’S LOST",

		"MACBETH",

		"MEASURE FOR MEASURE",

		"THE MERCHANT OF VENICE",

		"THE MERRY WIVES OF WINDSOR",

		"A MIDSUMMER NIGHT’S DREAM",

		"MUCH ADO ABOUT NOTHING",

		"OTHELLO, THE MOOR OF VENICE",

		"PERICLES, PRINCE OF TYRE",

		"KING RICHARD THE SECOND",

		"KING RICHARD THE THIRD",

		"ROMEO AND JULIET",

		"THE TAMING OF THE SHREW",

		"THE TEMPEST",

		"TIMON OF ATHENS",

		"TITUS ANDRONICUS",

		"TROILUS AND CRESSIDA",

		"TWELFTH NIGHT: OR, WHAT YOU WILL",

		"THE TWO GENTLEMEN OF VERONA",

		"THE TWO NOBLE KINSMEN",

		"THE WINTER’S TALE",

		"A LOVER’S COMPLAINT",

		"THE PASSIONATE PILGRIM",

		"THE PHOENIX AND THE TURTLE",

		"THE RAPE OF LUCRECE",
		"VENUS AND ADONIS",
	}

	searcher := Searcher{}
	searcher.Plays = plays

	err := searcher.Load("completeworks.txt")
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	http.HandleFunc("/search", handleSearch(searcher))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	fmt.Printf("Listening on port %s...", port)
	err = http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
	if err != nil {
		log.Fatal(err)
	}
}

type Searcher struct {
	CompleteWorks string
	Plays         []string
	SuffixArray   *suffixarray.Index
}

func handleSearch(searcher Searcher) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		query,
			ok := r.URL.Query()["q"]
		if !ok || len(query[0]) < 1 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("missing search query in URL params"))
			return
		}

		//Since the list shakesphere's plays are known
		//check if the input play query matches any of the plays defined
		// It contains the list of plays from the complete works txt
		// A user may not search exactly the title from the txt files
		// There any query that is contained in the play title defined is
		// associated with the play
		play := searcher.checkPlay(query[0], searcher.Plays)

		results := searcher.Search(play, searcher.Plays, searcher.CompleteWorks)

		buf := &bytes.Buffer{}

		enc := json.NewEncoder(buf)
		err := enc.Encode(results)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("encoding failure"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
	}
}

func (s *Searcher) Load(filename string) error {

	dat,
		err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("Load: %w", err)
	}

	s.CompleteWorks = string(dat)
	s.SuffixArray = suffixarray.New(dat)
	return nil
}

func (s *Searcher) Search(play string, plays []string, data string) string {

	//Retrieve indices for an instance of the play in the txt
	currentplayIndices := s.lookupPlay(play, data)

	//Retrieve indices for an instance of the next play in the txt
	nextplayIndices := []int{}
	//Output result text
	text := ""

	if len(currentplayIndices) != 0 {

		for i := 0; i < len(plays); i++ {
			if play == plays[i] {
				if i >= 41 {
					nextplayIndices = s.lookupPlay(plays[i], data)
				} else {
					nextplayIndices = s.lookupPlay(plays[i+1], data)
				}

			}
		}

		//Sort the indices for currentplay and next play
		// This allows use to retrieving the second instances of each play
		// The second instances allows us to retrieves the plays content in between the
		// two titles
		currentplayIndices, nextplayIndices = s.sortPlays(currentplayIndices, nextplayIndices)

		//For TWELFTH NIGHT: OR, WHAT YOU WILL
		if currentplayIndices[0] == 5040318 {
			text = data[currentplayIndices[0]:nextplayIndices[1]]
		} else if currentplayIndices[0] == 3615472 {
			//FOR OTHELLO, THE MOOR OF VENICE
			text = data[currentplayIndices[0]:nextplayIndices[1]]
		} else if currentplayIndices[1] == 3463991 {
			//  FOR MUCH ADO ABOUT NOTHING
			text = data[currentplayIndices[1]:nextplayIndices[0]]
		} else if currentplayIndices[1] == 4874441 {
			//  FOR MUCH ADO ABOUT NOTHING
			text = data[currentplayIndices[1]:nextplayIndices[0]]
		} else if currentplayIndices[1] == 1323176 {
			//THE SECOND PART OF KING HENRY THE FOURTH
			//The next index for the play THE LIFE OF KING HENRY THE FIFTH play cannot be found
			// There the index for the play  after KING HENRY THE FIFTH is used to retrieve the text
			text = data[currentplayIndices[1]:1651327]
		} else if currentplayIndices[0] == 2890 {
			//First instance of VENUS AND ADONIS
			firstText := data[currentplayIndices[1]:currentplayIndices[2]]
			//Second instance of VENUS AND ADONIS
			secondText := data[currentplayIndices[2]:5745664]
			text = firstText + "\n \n \n \n" + secondText
		} else {
			text = data[currentplayIndices[1]:nextplayIndices[1]]
		}
	} else {
		text = "No Play Found"
	}

	return text
}

func (s *Searcher) lookupPlay(play string, data string) []int {

	index := s.SuffixArray
	return index.Lookup([]byte(strings.ToUpper(play)), -1)
}

func (s *Searcher) sortPlays(currentplayIndices []int, nextplayIndices []int) ([]int, []int) {

	sort.Ints(currentplayIndices)
	sort.Ints(nextplayIndices)
	return currentplayIndices, nextplayIndices
}

func (s *Searcher) checkPlay(query string, plays []string) string {
	play := ""

	for _, v := range plays {
		if strings.Contains(v, strings.ToUpper(query)) {
			play = v
		}
	}

	return play
}
