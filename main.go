package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/blevesearch/bleve"
	"github.com/muesli/gamut"
	"github.com/psykhi/wordclouds"
)

type post struct {
	Title string    `json:"title"`
	Date  time.Time `json:"date"`
	Desc  string    `json:"desc"`
}

func main() {
	if len(os.Args) == 1 {
		flag.Usage()
	}
	switch os.Args[1] {
	case "index":
		index()
	case "search":
		fs := flag.NewFlagSet("search", flag.ExitOnError)
		q := fs.String("q", "", "The value to search for.")
		fs.Parse(os.Args[2:])
		search(*q)
	case "cloud":
		fs := flag.NewFlagSet("cloud", flag.ExitOnError)
		q := fs.String("q", "", "The value to search for.")
		fs.Parse(os.Args[2:])
		cloud(*q)
	default:
		fmt.Println("reinventrecap index|search|cloud")
	}
}

func search(q string) {
	fmt.Printf("Searching for %q\n", q)
	index, err := bleve.Open("recap.bleve")
	if err != nil {
		fmt.Println("failed to load index:", err)
		os.Exit(1)
	}
	query := bleve.NewMatchQuery(q)
	search := bleve.NewSearchRequest(query)
	search.Size = 50
	searchResults, err := index.Search(search)
	if err != nil {
		fmt.Println("failed to search index:", err)
		os.Exit(1)
	}
	fmt.Println(searchResults.String())
}

func load() (posts []post, err error) {
	f, err := os.Open("aws_releases.json")
	if err != nil {
		err = fmt.Errorf("failed to find JSON (have you ran 'index' yet?): %w", err)
		return
	}
	s := bufio.NewScanner(f)
	for s.Scan() {
		posts = append(posts, post{})
		json.Unmarshal([]byte(s.Text()), &posts[len(posts)-1])
	}
	return
}

var stopwords = []string{
	"i",
	"me",
	"my",
	"myself",
	"we",
	"our",
	"ours",
	"ourselves",
	"you",
	"your",
	"yours",
	"yourself",
	"yourselves",
	"he",
	"him",
	"his",
	"himself",
	"she",
	"her",
	"hers",
	"herself",
	"it",
	"its",
	"itself",
	"they",
	"them",
	"their",
	"theirs",
	"themselves",
	"what",
	"which",
	"who",
	"whom",
	"this",
	"that",
	"these",
	"those",
	"am",
	"is",
	"are",
	"was",
	"were",
	"be",
	"been",
	"being",
	"have",
	"has",
	"had",
	"having",
	"do",
	"does",
	"did",
	"doing",
	"a",
	"an",
	"the",
	"and",
	"but",
	"if",
	"or",
	"because",
	"as",
	"until",
	"while",
	"of",
	"at",
	"by",
	"for",
	"with",
	"about",
	"against",
	"between",
	"into",
	"through",
	"during",
	"before",
	"after",
	"above",
	"below",
	"to",
	"from",
	"up",
	"down",
	"in",
	"out",
	"on",
	"off",
	"over",
	"under",
	"again",
	"further",
	"then",
	"once",
	"here",
	"there",
	"when",
	"where",
	"why",
	"how",
	"all",
	"any",
	"both",
	"each",
	"few",
	"more",
	"most",
	"other",
	"some",
	"such",
	"no",
	"nor",
	"not",
	"only",
	"own",
	"same",
	"so",
	"than",
	"too",
	"very",
	"s",
	"t",
	"can",
	"will",
	"just",
	"don",
	"should",
	"now",
}

var stopWordsMap = map[string]struct{}{}

func init() {
	for _, w := range stopwords {
		stopWordsMap[w] = struct{}{}
	}
}

var versionRegexp = regexp.MustCompile(`v\d+\.\d+`)

func ignoreWord(s string) bool {
	if s == "" {
		return true
	}
	if _, inSkipMap := stopWordsMap[s]; inSkipMap {
		return true
	}
	if versionRegexp.MatchString(s) {
		return true
	}
	return false
}

func removePunctuationAndNumbers(s string) string {
	var op string
	for _, ss := range s {
		if unicode.IsPunct(ss) || unicode.IsNumber(ss) {
			continue
		}
		op += string(ss)
	}
	return op
}

func tidyWord(s string) string {
	s = removePunctuationAndNumbers(s)
	s = strings.ToLower(s)
	return s
}

func cloud(q string) {
	posts, err := load()
	if err != nil {
		fmt.Println("failed to load posts", err)
		os.Exit(1)
	}
	wordCounts := map[string]int{}

	shouldIncludePost := func(title string) bool {
		return true
	}

	if q != "" {
		fmt.Printf("creating filtered word cloud: %q\n", q)
		index, err := bleve.Open("recap.bleve")
		if err != nil {
			fmt.Println("failed to load index:", err)
			os.Exit(1)
		}
		query := bleve.NewMatchQuery(q)
		search := bleve.NewSearchRequest(query)
		search.Size = 200
		searchResults, err := index.Search(search)
		if err != nil {
			fmt.Println("failed to search index:", err)
			os.Exit(1)
		}
		postIDs := map[string]struct{}{}
		for _, sr := range searchResults.Hits {
			postIDs[sr.ID] = struct{}{}
		}
		shouldIncludePost = func(title string) bool {
			_, ok := postIDs[title]
			return ok
		}
	}

	var postCount int
	for _, p := range posts {
		if !shouldIncludePost(p.Title) {
			continue
		}
		for _, s := range strings.Split(p.Title, " ") {
			for _, ss := range strings.Split(s, "-") {
				ss := tidyWord(ss)
				if !ignoreWord(ss) {
					wordCounts[ss] = wordCounts[ss] + 1
				}
			}
		}
		for _, s := range strings.Split(p.Desc, " ") {
			for _, ss := range strings.Split(s, "-") {
				ss := tidyWord(ss)
				if !ignoreWord(ss) {
					wordCounts[ss] = wordCounts[ss] + 1
				}
			}
		}
		postCount++
	}
	fmt.Printf("creating word cloud from %d posts\n", postCount)
	colors, err := gamut.Generate(8, gamut.PastelGenerator{})
	if err != nil {
		fmt.Println("failed to generate colors", err)
		os.Exit(1)
	}
	w := wordclouds.NewWordcloud(
		wordCounts,
		wordclouds.FontFile("./roboto-regular.ttf"),
		wordclouds.Height(2048),
		wordclouds.Colors(colors),
		wordclouds.Width(2048),
	)
	img := w.Draw()
	outputFile, err := os.Create("./wordcloud.png")
	if err != nil {
		fmt.Println("failed to create wordcloud.png file:", err)
		os.Exit(1)
	}
	defer outputFile.Close()
	err = png.Encode(outputFile, img)
	if err != nil {
		fmt.Println("failed to encode PNG:", err)
		os.Exit(1)
	}
}

func index() {
	// Read data.
	f, err := os.Open("./aws_releases.txt")
	if err != nil {
		fmt.Println("Failed to open file", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	posts := make([]post, 1)
	var lineIndex int
	for scanner.Scan() {
		line := scanner.Text()
		postIndex := lineIndex / 3
		switch lineIndex % 3 {
		case 0:
			posts[postIndex].Title = line
		case 1:
			posts[postIndex].Date, err = time.Parse("Jan 2, 2006", strings.TrimPrefix(line, "Posted On: "))
			if err != nil {
				fmt.Println("failed to parse time:", err)
			}
		case 2:
			posts[postIndex].Desc = line
			posts = append(posts, post{})
		}
		lineIndex++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	// Write out JSON.
	w, err := os.Create("./aws_releases.json")
	if err != nil {
		fmt.Println("failed to create file", err)
		os.Exit(1)
	}
	defer w.Close()
	for _, p := range posts {
		jj, err := json.Marshal(p)
		if err != nil {
			fmt.Println("failed to marshal JSON", err)
			os.Exit(1)
		}
		_, err = w.Write(append(jj, '\n'))
		if err != nil {
			fmt.Println("failed to write", err)
			os.Exit(1)
		}
	}

	// Index the data.
	mapping := bleve.NewIndexMapping()
	index, err := bleve.New("recap.bleve", mapping)
	if err != nil {
		fmt.Println("failed to create index", err)
		os.Exit(1)
	}
	for _, p := range posts {
		p := p
		if p.Title != "" {
			err = index.Index(p.Title, p)
			if err != nil {
				fmt.Println("error indexing data:", err)
			}
		}
	}
}
