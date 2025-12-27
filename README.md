<img src="https://raw.githubusercontent.com/1xayd1/lightlang/refs/heads/main/logo.svg" 
     width="100" 
     alt="Logo">
# lightlang
lightlang is a fairly fast and compact language implemented in golang.
It's supposed to incorporate 3 syntax styles from other languages such as:
luau, typescript, javascript, golang and others...

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
