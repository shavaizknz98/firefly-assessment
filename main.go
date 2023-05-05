package main

import (
	"bufio"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const WordBankUrl = "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt"

type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

/*
Objective:
In this assignment, you have to fetch this list of essays and count the top 10 words from all the
essays combined.
A valid word will:
1. Contain at least 3 characters.
2. Contain only alphabetic characters.
3. Be part of our bank of words (not all the words in the bank are valid according to the
previous rules)
The output should be pretty json printed to the stdout.

My solution:
At first glance it seems like a simple dictionary problem,
Initial guess would be to fetch the list of essays, get the articleBody and
clean it up (remove punctuation, special characters, etc),
then tokenize the articleBody into words and validate each word against the rules,
store each word in a map with value as the count of the word,
finally sort the map by value and print the top 10 words.

This can be improved by fetching the list of essays concurrently, and processing each essay concurrently,
the dictionary can be a "global" map that is shared by all the goroutines, and we can use a mutex to lock when writing.


There are some considerations to be made:
1. Cannot spin up too many goroutines as this could cause memory issues and also cause rate limiting
2. Cannot make too many requests at once as well, as again this could cause rate limiting

Solutions for the above are:
1. Spin up max 2000 goroutines
2. Add a random sleep between 200-100msec before initiating a request so that not all requests are made at once

However due to engadgets policies you may still be rate limited if you run the script too often at once, in that case a log is placed
*/

func main() {
	// Get word bank from URL given in assignment
	wordBank := getWordBank(WordBankUrl)
	log.Println("Number of words in word bank: ", len(*wordBank))

	// Get list of essay URLs
	essays := getEssays("./endg-urls.txt")
	log.Println("Number of essays: ", len(*essays))

	// Compile regex for valid words so that it can be used later
	regExpression := regexp.MustCompile(`\b[a-z]{3,}\b`)
	wordMap := make(map[string]int, 0)

	// Concurrently process 2000 essays at a time to avoid too many goroutines and rate limit issues with engagdet
	essayBatch := make([][]string, 0)
	for i := 0; i < len(*essays); i += 2000 {
		endBatch := i + 2000
		if endBatch > len(*essays) {
			endBatch = len(*essays)
		}
		essayBatch = append(essayBatch, (*essays)[i:endBatch])
	}

	// Fetch essays concurrently, 2000 at a time, then wait for all to finish then process next batch
	for i, batch := range essayBatch {
		// Wait group will have length max 2000 at a time, wait for all to finish before proceeding to next batch
		var wg sync.WaitGroup
		wg.Add(len(batch))

		//mutex is needed so we can write to our hashmap concurrently without issues
		mtx := sync.Mutex{}

		// Loop over essay URL, fetch Essay and extract valid words from each essay. Then check if the valid words is within the word bank
		for _, essayUrl := range batch {
			go func(essayUrl string) {
				defer wg.Done()
				words := fetchWordsFromEssay(essayUrl, wordBank, regExpression)

				mtx.Lock()
				processEssay(&wordMap, words)
				mtx.Unlock()
			}(essayUrl)
		}
		wg.Wait()
		log.Println("Finished batch", i+1, "out of", len(essayBatch))
	}

	wordMap = *sortWordMap(&wordMap)

	prettyJson, err := json.MarshalIndent(wordMap, "", "  ")
	if err != nil {
		log.Fatal(err)
	}

	log.Println(string(prettyJson))
}

func processEssay(wordMap *map[string]int, words *[]string) {
	for _, word := range *words {
		(*wordMap)[word]++
	}
}

func fetchWordsFromEssay(essayUrl string, wordBank *map[string]struct{}, regExpression *regexp.Regexp) *[]string {
	// sleep for random amount of time between 200-1000 msec to avoid being rate limited
	time.Sleep(time.Duration(rand.Intn(800)+200) * time.Millisecond)

	var validEssayWords []string
	req, err := http.Get(essayUrl)
	if err != nil {
		log.Fatal(err)
	}

	defer req.Body.Close()

	htmlFile, err := html.Parse(req.Body)
	if err != nil {
		log.Fatal(err)
	}

	// Traverse the html nodes and get the articleBody that is inside <script type="application/ld+json">
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, a := range n.Attr {
				if a.Key == "type" && a.Val == "application/ld+json" {
					// Parse the json and get the articleBody by marshalling it into a map
					var m map[string]interface{}
					err := json.Unmarshal([]byte(n.FirstChild.Data), &m)
					if err != nil {
						log.Fatal(err)
					}

					if _, ok := m["articleBody"]; !ok {
						log.Println("articleBody not found in", essayUrl)
						return
					}

					articleBody := m["articleBody"].(string)
					essayWords := regExpression.FindAllString(articleBody, -1)
					for _, word := range essayWords {
						if _, ok := (*wordBank)[word]; ok {
							validEssayWords = append(validEssayWords, word)
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(htmlFile)

	// if validEssayWords is empty, then we are likely being ratelimited
	if len(validEssayWords) == 0 {
		log.Println("No words found in", essayUrl, "likely being rate limited")
	}

	return &validEssayWords
}

func getWordBank(url string) *map[string]struct{} {
	wordBank := map[string]struct{}{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		word := strings.ToLower(scanner.Text())
		wordBank[word] = struct{}{}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return &wordBank
}

func getEssays(filePath string) *[]string {
	var essays []string

	// Fetch from local file
	f, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}

	essays = strings.Split(string(f), "\n")

	return &essays
}

func sortWordMap(wordMap *map[string]int) *map[string]int {
	var wordMapSlice []WordCount
	for k, v := range *wordMap {
		wordMapSlice = append(wordMapSlice, WordCount{k, v})
	}

	/* a custom sorting algorithm can be used here to sort by value considering it will use builtins instead,
	however the underlying logic would be the same so this is fine for purposes of the exercise */
	sort.Slice(wordMapSlice, func(i, j int) bool {
		return wordMapSlice[i].Count > wordMapSlice[j].Count
	})

	// get top 10 only
	sortedWordMap := make(map[string]int, 10)
	for i, wordCount := range wordMapSlice {
		sortedWordMap[wordCount.Word] = wordCount.Count
		if i == 9 {
			break
		}
	}

	return &sortedWordMap
}
