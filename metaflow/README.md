Requirements:
* python3
* docker

Configure environment for local metaflow runs.

```
python -m venv virtual-env
```

```
source virtual-env/bin/activate
```

```
pip install --require-hashes -r requirements.txt
```


```
metaflow-dev up
```

Use local environment.

```
metaflow-dev shell
source virtual-env/bin/activate
```