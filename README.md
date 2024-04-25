# GOT
Like Git, but in Go.

Got is a command-line application developed in Go, functioning as a basic version control system similar to Git. It allows users to perform essential operations like initializing repositories, staging changes, committing snapshots, and checking out versions, leveraging Go’s robust file handling and command-line utilities.

## Purpose

1. Develop a deeper understanding of how Git works
2. Build proficiency with Go, particularly working with files

 ## Version control operations

   - **Repository Initialization (`init` command):** Sets up a new repository by creating necessary directory structures and initializing a HEAD file, which tracks the current branch.

   - **Staging Changes (`add` and `remove` commands):** Manages the staging area, where changes are prepped for commits. Involves updating the index with file statuses.

   - **Committing Changes (`commit` command):** Takes a snapshot of the staged changes, creating a commit object that includes metadata like the commit message and parent commit. When committed, files are compressed (using zlib) and this snapshot can be identified by the resulting SHA-1 hash.
     
   - **Checkout Feature (`checkout` command):** Allows users to revert their working directory to the state of a specific commit, identified by its hash.


## Built using
- The Go standard libary

## Technical features
- File handling
- Compression and decompression
- Hashing
- Unit testing
- Custom data structures
