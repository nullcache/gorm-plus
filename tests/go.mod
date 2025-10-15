module github.com/nullcache/gorm-plus/tests

go 1.24.0

require (
	github.com/nullcache/gorm-plus v0.1.0
	github.com/stretchr/testify v1.11.1
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.30.5
)

replace github.com/nullcache/gorm-plus => ../

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
