# ⛨ Buckler ⛨

[![Buckler Shield](http://b.repl.ca/v1/use-buckler-blue.png)](http://buckler.repl.ca)
[![MIT License](http://gitshields.com/v1/text/license/MIT/red.png)](LICENSE)

Buckler is [Shields](https://github.com/badges/shields) as a Service (ShaaS, or alternatively, Badges as a Service)
for use in GitHub READMEs, or anywhere else. Use buckler with your favorite continuous integration tool, performance
monitoring service API, or ridiculous in-joke to surface information.

# API

Buckler tries to make creating shields easy. Each shield request is a url that has four parts:
- `type`
- `key`
- `value`
- `color`

Parts are separated by forward slashes. The request is suffixed by `.png` and prefixed with the Buckler host and API version, `gitshields.com/v1/`

Drone shields take two colors seperated by a hyphen for the passing and failing color.

## Examples

- http://gitshields.com/v1/text/text/example/brightgreen.png ⇨ ![](http://gitshields.com/v1/text/text/example/brightgreen.png)
- http://gitshields.com/v1/pypi/setuptools/version/green.png ⇨ ![](http://gitshields.com/v1/pypi/setuptools/version/green.png)
- http://gitshields.com/v1/pypi/setuptools/day_down/orange.png ⇨ ![](http://gitshields.com/v1/pypi/setuptools/day_down/orange.png)
- http://gitshields.com/v1/pypi/setuptools/week_down/red.png ⇨ ![](http://gitshields.com/v1/pypi/setuptools/week_down/red.png)
- http://gitshields.com/v1/pypi/setuptools/month_down/lightgrey.png ⇨ ![](http://gitshields.com/v1/pypi/setuptools/month_down/lightgrey.png)
- http://gitshields.com/v1/drone/pritunl/pritunl/brightgreen-red.png ⇨ ![](http://gitshields.com/v1/drone/pritunl/pritunl/brightgreen-red.png)

## Valid Colours

- `brightgreen` ⇨ ![](http://gitshields.com/v1/text/colour/brightgreen/brightgreen.png)
- `green` ⇨ ![](http://gitshields.com/v1/text/colour/green/green.png)
- `yellowgreen` ⇨ ![](http://gitshields.com/v1/text/colour/yellowgreen/yellowgreen.png)
- `yellow` ⇨ ![](http://gitshields.com/v1/text/colour/yellow/yellow.png)
- `orange` ⇨ ![](http://gitshields.com/v1/text/colour/orange/orange.png)
- `red` ⇨ ![](http://gitshields.com/v1/text/colour/red/red.png)
- `grey` ⇨ ![](http://gitshields.com/v1/text/colour/grey/grey.png)
- `lightgrey` ⇨ ![](http://gitshields.com/v1/text/colour/lightgrey/lightgrey.png)
- `blue` ⇨ ![](http://gitshields.com/v1/text/colour/blue/blue.png)

Six digit RGB hexidecimal colour values work as well:

- `804000` - ![](http://gitshields.com/v1/text/colour/brown/804000.png)

### Grey?

Don't worry; `gray` and `lightgray` work too.

## URL Safe

Buckler API requests are just HTTP GETs, so remember to URL encode!

http://gitshields.com/v1/text/uptime/99.99%25/yellowgreen.png ⇨ ![](http://gitshields.com/v1/text/uptime/99.99%25/yellowgreen.png)

# Try It Out

Play around with the simple form on [gitshields.com](http://gitshields.com/)

# Installing

```bash
go get github.com/zachhuff386/buckler
```

Alternatively, `git clone` and `go build` to run from source.

# Thanks

- James Bowes for the orignal [buckler](https://github.com/badges/buckler) project
- Olivier Lacan for the [shields](https://github.com/badges/shields) repo
- Steve Matteson for [Open Sans](http://opensans.com/)
