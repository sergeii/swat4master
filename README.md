# SWAT4 Master Server

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/sergeii/swat4master.svg)](https://github.com/sergeii/swat4master)
![https://github.com/sergeii/swat4master/actions/workflows/ci.yml?query=branch%3Amain+](https://github.com/sergeii/swat4master/actions/workflows/ci.yml/badge.svg?branch=main)
[![codecov](https://codecov.io/gh/sergeii/swat4master/branch/main/graph/badge.svg?token=ZYQ1x62kR3)](https://codecov.io/gh/sergeii/swat4master)
[![Go Report Card](https://goreportcard.com/badge/github.com/sergeii/swat4master)](https://goreportcard.com/report/github.com/sergeii/swat4master)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/007d7e28f8ba4f63a56dc1bd095bb2b2)](https://www.codacy.com/gh/sergeii/swat4master/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=sergeii/swat4master&amp;utm_campaign=Badge_Grade)

## Description
This project implements the GameSpy master server protocol
that is fully compatible with SWAT4 multiplayer.
Namely, it accepts heartbeat requests from game servers
and allows players to browse these servers from the in-game server list.

Backed by this project and widely accepted in the community,
the master server is available for use by players and server owners either with [a patch](https://github.com/sergeii/swat-patches/tree/master/swat4stats-masterserver) or
with a [hosts](https://www.howtogeek.com/howto/27350/beginner-geek-how-to-edit-your-hosts-file/) file adjustment:
```
116.202.1.82 swat4.available.gamespy.com
116.202.1.82 swat4.master.gamespy.com
116.202.1.82 swat4.ms15.gamespy.com
```

## Background
GameSpy shut down its services in 2014, rendering multiplayer for [a good share of games](https://www.reddit.com/r/Games/comments/22fz75/list_of_games_affected_by_gamespy_shutdown/) unusable.
For SWAT4, however, [it happened a year earlier](https://www.pcgamer.com/gamespy-shuts-down-multiplayer-support-for-swat-4-neverwinter-nights-and-other-classics/).

In 2013, I launched [swat4stats.com](https://swat4stats.com/) [[GitHub](https://github.com/sergeii/swat4stats.com)],
a player statistics tracking service for SWAT4. The core feature set in [swat4stats.com](https://swat4stats.com/)
has always been about statistics and numbers. However, one of its extra features, the live server browser,
has quickly become the most popular part of the service thanks to the GameSpy shutdown.

<img src="https://user-images.githubusercontent.com/4739840/164216907-1d69d6d5-558c-4c96-9533-7e616911f8e7.png" alt="drawing" width="600" />


A couple of years later, with the help of the SWAT4 community and research articles published by [Luigi Auriemma](http://aluigi.altervista.org/papers.htm#distrust),
I was able to reverse engineer the protocols used by the game, and then reimplement the master server functionality,
returning servers back to the in-game server browser:

<img src="https://user-images.githubusercontent.com/4739840/164222220-53200246-1a58-497f-9694-6dd811a786c3.png" alt="drawing" width="600" />

## Usage
If for any reason you wish to run your own instance of this service you can do it using a docker container:
```
docker run --rm ghcr.io/sergeii/swat4master:latest
```
For other tags see [container registry](https://github.com/sergeii/swat4master/pkgs/container/swat4master/versions).

Alternatively you can download and run a server binary suitable for your platform from one of the [releases](https://github.com/sergeii/swat4master/releases).

## Building from source
To build the project from source you need Go 1.18+
```
go build -o swat4master cmd/swat4master/main.go
```
