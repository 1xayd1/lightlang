<img src="https://raw.githubusercontent.com/1xayd1/lightlang/refs/heads/main/logo.svg" 
     width="100" 
     alt="Logo">
# lightlang
A simple experiment language written in 2 days (of editor time) in golang.
I thought it'd be nice to make something like luau typescript python and other languages in golang so i decided to make this;

To build your own version of the project use build.bat file:
```
	.\build.bat 
```


To get compiled bytecode of your files:
```
	lightlang build example.ll
```
This will output .llbytecode file with the name of the original file e.g:
```
	'example.ll' -> 'example.llbytecode'
```


To run your files directly:
```
	lightlang run example.ll
```
