# dupes
A tool for finding duplicate files

# How to build
```
dep ensure
go build dupes.go
```

# How to run
`./dupes DIRECTORY`

The DIRECTORY argument should be a directory. dupes will recursively walk all of the files in all subdirectories print out any duplicate files.

dupes uses a dual hash to ensure collisions of a single hash do not result in false positive duplicates. Currently, xxhash is used as the primary hash, with highwayhash used as the secondary hash to verify duplicates.