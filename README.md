Steps to run

brew install go  
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps  
go run .  

OR  
brew install go  
go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps  
go build .  
./Go-playwright 


If you are realling feeling adventurous you use the following command to flip the image naming convention so sizes come first, domains second.  
./Go-playwright -flip true


See the "out" folder for your screenshots.  
Modify the urls.txt file to add your urls.  
Modify the sizes.json to add custom sizes.
