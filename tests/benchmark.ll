let start = tick()

let i = 0
while i < 50000000 do
    i = i + 1
end

let end_time = tick()
let elapsed = end_time - start

print("Sum: " + i)
print("Elapsed time: " + elapsed + " seconds")
print("Iterations per second: " + 1000000 / elapsed)
