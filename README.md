# SpectroDashboard
Dashboard for metal spectrometer results

## Build
- Run `go get github.com/RoanBrand/SpectroDashboard`
- Copy the following to a folder where the service will live:  
  - `SpectroDashboard` executable from `$GOPATH/bin`
  - `static` folder
  - `config.json`
  
## Run
- Configure your `config.json`
- You can run the service in a terminal, or:
- You can install it as a OS service with `SpectroDashboard.exe -service install`
- You can also remove the service with `-service uninstall`

### Notes
- Tested with Go 1.10
- Windows 7  x86 uses DataSource String `Provider=Microsoft.Jet.OLEDB.4.0;`
- Windows 10 x64 uses DataSource String `Provider=Microsoft.ACE.OLEDB.12.0;`
