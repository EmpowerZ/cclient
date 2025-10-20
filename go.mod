module github.com/EmpowerZ/cclient

go 1.23.0

toolchain go1.23.8

require (
	github.com/EmpowerZ/fhttp v0.0.0-20251013161504-7f610adce28e
	github.com/refraction-networking/utls v1.6.7
	golang.org/x/net v0.43.0
	golang.org/x/sync v0.16.0
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
)

replace github.com/refraction-networking/utls => github.com/EmpowerZ/utls v0.0.0-20250409141327-3ccc193b058e
