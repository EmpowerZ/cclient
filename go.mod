module github.com/EmpowerZ/cclient

go 1.21

toolchain go1.21.5

require (
	github.com/EmpowerZ/fhttp v0.0.0-20250409145910-4f35366bf228
	github.com/refraction-networking/utls v1.6.7
	golang.org/x/net v0.23.0
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/refraction-networking/utls => github.com/EmpowerZ/utls v0.0.0-20250409141327-3ccc193b058e
