# SpectroDashboard
Dashboard for metal spectrometer results

## Dependencies
- Go 11.1+

## Build
- Clone or download this repository.
- Run `go build cmd\SpectroDashboardMDB\SpectroDashboardMDB.go` inside.
- Run `go build cmd\SpectroDashboardXML\SpectroDashboardXML.go`
- Copy the following to a folder where the services will live:  
  - `SpectroDashboard` executable`
  - `static` folder
  - `config.json`
  
## Run
- Configure your `config.json`
- You can run the service in a terminal, or:
- You can install it as a OS service with `SpectroDashboardXXX.exe -service install`
- You can also remove the service with `-service uninstall`

### Notes
- Windows 7  x86 uses DataSource String `Provider=Microsoft.Jet.OLEDB.4.0;`
- Windows 10 x64 uses DataSource String `Provider=Microsoft.ACE.OLEDB.12.0;`
- To build for 32 bit, make sure you run `cmd`, followed by `set GOARCH=386`, before running `go build`
