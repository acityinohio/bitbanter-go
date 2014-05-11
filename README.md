# bitbanter-go readme

This repository contains the old code for bitbanter, which ran on golang and Google App engine (now my personal blog and running through the magic of GitHub Pages). To run locally, install the Google App Engine tools and Go. While in the root directory, run:

	dev_appserver.py .

Should appear on localhost:8080.

Note: you need a number of API keys/constants for both Twitter and Coinbase in order for everything to work (put these into the placeholders within ./banter/kekeke/kekeke.go). The Coinbase API has updated since I initially built this, but I likely won't bother to update to the non-deprecated API since I'm no longer actively developing bitbanter-go. Still, I figured I would open source the code in case anyone was interested.

Additional note: I am merely a hobbyist programmer, not a pro, so I apologize in advance for the overall design of my code.
