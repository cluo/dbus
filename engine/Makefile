all:

bench:
	@go test -v -benchmem -benchtime=30s -test.bench=".*" -cpuprofile cpu.pprof
	@go tool pprof engine.test cpu.pprof

v2:
	go build -tags v2

escape:
	go build -gcflags '-m=1'
