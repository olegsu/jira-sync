echo "Cleaning old logs directory"
rm -rf $PWD/logs/* || true
echo "Building binary"
go build -o dist/jira-sync .
echo "Running..."
./trello-sync