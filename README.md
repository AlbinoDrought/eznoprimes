# eznoprimes

Track your way to partner plus

- Loads subcount from a text file
- Adds 1 to the text file when a non-gifted and non-prime sub or resub is received
- Overwrites the text file when a mod or broadcaster sends a message like "!nonprimesubcount 0"

This is intended to be used with an OBS "Read from file" text source.

## Configuration

Run the program to print an empty config to the console.

Fill out and save this config to `config.json` (or specify an alternate path using `EZNOPRIMES_CONFIG_PATH=somewhere/else.json`), and run the program again.
