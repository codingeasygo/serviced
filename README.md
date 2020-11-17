Serviced
===
### Install(Windows)
```.bat
go get github.com/codingeasygo/serviced/serviced
serviced install
```

### Install(Linux)
```.sh
go get github.com/codingeasygo/serviced/serviced
cp $GOPATH/bin/serviced /usr/bin/serviced
mkdir /home/serviced/
cp -f $GOPATH/srv/github.com/codingeasygo/serviced/serviced/serviced.service /etc/systemd/system/
systemctl enable serviced
systemctl start serviced
```

### ServiceD configure file
```.json
{
  "includes": {
    "test-config.json": 1
  }
}
```

### Service Group Configure File
```.json
{
    "name": "example",
    "services": [
        {
            "name": "service name",
            "path": "service executable",
            "args": [
                "arguments"
            ],
            "env": [
                "env=1"
            ],
            "dir": "working directory",
            "stderr": "std error output file",
            "stdout": "std normal output file"
        },
        {
            "name": "service name",
            "path": "${CONF_DIR}/using environment value",
            "args": [
                "${CONF_DIR}/arguments"
            ],
            "env": [
                "env=${CONF_DIR}"
            ],
            "dir": "${ENV Value}/working directory",
            "stderr": "std error output file",
            "stdout": "std normal output file"
        }
    ]
}
```

### Usage
* `serviced add <group configure file>` add group service
* `serviced remove <group name>` remove group service
* `serviced start <group name>` start group service
* `serviced stop <group name>` stop group service