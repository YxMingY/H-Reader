# ECDICT

`ecdict.csv` is derived from the base dictionary from
[skywind3000/ECDICT](https://github.com/skywind3000/ECDICT/blob/master/ecdict.csv).
It retains 404,012 entries whose headword contains no internal whitespace,
including entries with hyphens or apostrophes. The 366,599 multiword entries
were removed. Retained CSV records are unchanged.
The upstream [license](https://github.com/skywind3000/ECDICT/blob/master/LICENSE)
is included in the application assets at `/licenses/ECDICT.txt`.

The application embeds the CSV and builds an in-memory index of word and
inflection offsets. Lookups use the Chinese translation and phonetic fields.
