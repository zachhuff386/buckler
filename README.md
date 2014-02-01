# Git Shields

[![Git Shields](http://gitshields.com/v2/text/git/shields/blue.png)](http://gitshields.com/)
[![MIT License](http://gitshields.com/v2/text/license/MIT/red.png)](LICENSE)

Git Shields is a [Shields](https://github.com/badges/shields) service for python projects based on [Buckler](https://github.com/badges/buckler) with support added for drone.io and pypi.

# API

Each url has three parts the seperated by forward slashes:

`http://gitshields.com/v2/:shield_type/:shield_parms/:shield_color.png`

#### shield type

Can be `text`, `pypi` or `drone`

#### shield params

Shield params are seperated by a forward slash and are different for each type

- `text` ⇨ `:right_text/:left_text`
- `pypi` ⇨ `:pypi_project/:query_type`
- `drone` ⇨ `:drone_project_path`

#### sheild color

Color can be one of the predefined colors below or a hex color code, `drone` badges require two colors separated by a hyphen for the build passing and failing color.

## Examples

- http://gitshields.com/v2/text/text/example/brightgreen.png ⇨ ![](http://gitshields.com/v2/text/text/example/brightgreen.png)
- http://gitshields.com/v2/pypi/setuptools/version/green.png ⇨ ![](http://gitshields.com/v2/pypi/setuptools/version/green.png)
- http://gitshields.com/v2/pypi/setuptools/day_down/orange.png ⇨ ![](http://gitshields.com/v2/pypi/setuptools/day_down/orange.png)
- http://gitshields.com/v2/pypi/setuptools/week_down/red.png ⇨ ![](http://gitshields.com/v2/pypi/setuptools/week_down/red.png)
- http://gitshields.com/v2/pypi/setuptools/month_down/lightgrey.png ⇨ ![](http://gitshields.com/v2/pypi/setuptools/month_down/lightgrey.png)
- http://gitshields.com/v2/drone/github.com/pritunl/pritunl/brightgreen-red.png ⇨ ![](http://gitshields.com/v2/drone/github.com/pritunl/pritunl/brightgreen-red.png)

## Valid Colours

- `brightgreen` ⇨ ![](http://gitshields.com/v2/text/colour/brightgreen/brightgreen.png)
- `green` ⇨ ![](http://gitshields.com/v2/text/colour/green/green.png)
- `yellowgreen` ⇨ ![](http://gitshields.com/v2/text/colour/yellowgreen/yellowgreen.png)
- `yellow` ⇨ ![](http://gitshields.com/v2/text/colour/yellow/yellow.png)
- `orange` ⇨ ![](http://gitshields.com/v2/text/colour/orange/orange.png)
- `red` ⇨ ![](http://gitshields.com/v2/text/colour/red/red.png)
- `grey` ⇨ ![](http://gitshields.com/v2/text/colour/grey/grey.png)
- `lightgrey` ⇨ ![](http://gitshields.com/v2/text/colour/lightgrey/lightgrey.png)
- `blue` ⇨ ![](http://gitshields.com/v2/text/colour/blue/blue.png)

Six digit RGB hexidecimal colour values work as well:

- `804000` - ![](http://gitshields.com/v2/text/colour/brown/804000.png)

## URL Safe

Buckler API requests are just HTTP GETs, so remember to URL encode!

http://gitshields.com/v2/text/uptime/99.99%25/yellowgreen.png ⇨ ![](http://gitshields.com/v2/text/uptime/99.99%25/yellowgreen.png)

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
