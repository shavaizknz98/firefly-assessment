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

To run the script:
Just run the precompiled binary in the root directory

```
./top-10-essay-word-counter
```
