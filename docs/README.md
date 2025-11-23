# Brainstorm

Brainstorm is a focused text transformation tool designed to help generate and normalize candidate strings from raw text. It is particularly useful for tasks like wordlist creation, passphrase / token derivation, and transforming free-form text into structured candidate outputs.

`Brainstorm` is written in `Go`, is compatible with multiple platforms, and is designed to work well in Unix-style pipelines. It reads from standard input and writes transformed output to standard output.

> Brainstorm is intentionally minimal. It focuses on:
> - N‑gram generation from sentences.
> - Filtering noisy or non-word-like input.
> - Generating normalized, case-adjusted tokens with configurable length ranges.

## Features

- **Streaming stdin pipeline:** Reads from standard input and writes to standard output, making it easy to chain with other tools.
- **N‑gram Generation:** Generates n‑grams over a configurable word-length range.
- **Normalization & Cleanup:**
  - Removes leading/trailing non-letter characters on each line.
  - Filters out lines that are unlikely to contain meaningful words.
  - Cleans common control and whitespace characters.
- **Case Transformations:**
  - Converts text to lowercase.
  - Generates title-cased concatenations (e.g., `"hello world"` → `"HelloWorld"`).
- **Length Filtering:** Keeps only outputs whose byte-length falls within a configurable range.
- **Parallel Processing:**
  - Uses a worker pool derived from `GOMAXPROCS` to process lines concurrently.

### Install

From source with `go`:

```bash
go install github.com/hashcracky/brainstorm@latest
```

From `git` clone then build with `go`:

```bash
git clone https://github.com/hashcracky/brainstorm
cd brainstorm
go build ./main.go
mv ./main ~/go/bin/brainstorm
brainstorm
```

---

## Basic Usage

Brainstorm reads from standard input and writes to standard output. There are no positional arguments; all behavior is controlled via flags.

```text
Usage of Brainstorm version (0.1.0):

input | brainstorm [options] > output

Accepts standard input and writes transformed output to standard output.
```

### Core Flags

- `-w string`
  - N‑gram word-length range in the form `start-end`.
  - Default: `1-5`
- `-l string`
  - Final output length range in the form `min-max`.
  - Default: `4-32`

Example:

```bash
cat source.txt | brainstorm -w 1-4 -l 6-20 > candidates.txt
```
