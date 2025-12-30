# gitexhume
`gitexhume` is a command line tool for scanning git repositories for sensitive. 
This is done by scanning keywords through the complete commit history

It can scan repositories directly from GitHub or work on repositories that
are already cloned locally.

---

## Requirements

- Go (1.20+ recommended)  
- Git  
- Linux or macOS  

---

## Setup

Before running the tool, the file `config.go` needs to be updated to include the `DefaultWordlist` path.

## Installation

Execute the following commands in the bash shell:

```bash
git clone https://github.com/DBeuken/gitexhume.git
cd gitexhume
go build -o gitexhume
```

## Usage

Scan all public repos of a GitHub user:

```bash
gitexhume -u username
```

Scan only selected repos:

```bash
gitexhume -u username -r repo1,repo2
```
