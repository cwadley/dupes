# dups
A tool for finding duplicate files

# How to build
```
dep ensure
go build dups.go
```

# How to run
`./dups DIRECTORY`

The DIRECTORY argument should be a directory. dups will recursively walk all of the files in all subdirectories print out any duplicate files, based on their xxhash values.