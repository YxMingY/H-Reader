# ECDICT

`ecdict.csv` is the unmodified base dictionary from
[skywind3000/ECDICT](https://github.com/skywind3000/ECDICT/blob/master/ecdict.csv).
The upstream [license](https://github.com/skywind3000/ECDICT/blob/master/LICENSE)
is included in the application assets at `/licenses/ECDICT.txt`.

The application embeds the CSV and builds an in-memory index of word and
inflection offsets. Lookups use the Chinese translation and phonetic fields.
