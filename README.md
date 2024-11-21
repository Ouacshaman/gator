# gator

Postgress is required to run the db because of packages compatibility
Go-lang needs to be installed to run the go app/package
To install gator do go build -o gator
have these in the go.mod file
github.com/google/uuid v1.6.0 // indirect
github.com/lib/pq v1.10.9 // indirect
make sure to goose up and goose down
use brew install for goose and sqlc
sqlc to generate the functions from querires in sql/queries/users.sql
