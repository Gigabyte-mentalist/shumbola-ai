# shumbola.ai ffuf image: secsi/ffuf ships no bundled wordlist (see
# FfufTool's defaultFfufWordlist), so this layer adds a small, built-in
# common-paths list to avoid requiring a volume mount at run time.
FROM secsi/ffuf:latest
COPY wordlist.txt /usr/share/wordlists/common.txt
