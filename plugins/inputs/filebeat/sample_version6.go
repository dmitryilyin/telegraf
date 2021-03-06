package filebeat

const filebeat6Info = `
{
  "beat": "filebeat",
  "hostname": "node-6",
  "name": "node-6-test",
  "uuid": "9c1c8697-acb4-4df0-987d-28197814f785",
  "version": "6.4.2"
}
`

const filebeat6Stats = `
{
  "beat": {
    "cpu": {
      "system": {
        "ticks": 626970,
        "time": {
          "ms": 626972
        }
      },
      "total": {
        "ticks": 5215010,
        "time": {
          "ms": 5215018
        },
        "value": 5215010
      },
      "user": {
        "ticks": 4588040,
        "time": {
          "ms": 4588046
        }
      }
    },
    "info": {
      "ephemeral_id": "809e3b63-4fa0-4f74-822a-8e3c08298336",
      "uptime": {
        "ms": 327248661
      }
    },
    "memstats": {
      "gc_next": 20611808,
      "memory_alloc": 12692544,
      "memory_total": 462910102088,
      "rss": 80273408
    }
  },
  "filebeat": {
    "events": {
      "active": 0,
      "added": 182990,
      "done": 182990
    },
    "harvester": {
      "closed": 2222,
      "open_files": 4,
      "running": 4,
      "skipped": 0,
      "started": 2226
    },
    "input": {
      "log": {
        "files": {
          "renamed": 0,
          "truncated": 0
        }
      }
    }
  },
  "libbeat": {
    "config": {
      "module": {
        "running": 0,
        "starts": 0,
        "stops": 0
      },
      "reloads": 0
    },
    "output": {
      "events": {
        "acked": 172067,
        "active": 0,
        "batches": 1490,
        "dropped": 0,
        "duplicates": 0,
        "failed": 0,
        "total": 172067
      },
      "read": {
        "bytes": 0,
        "errors": 0
      },
      "type": "kafka",
      "write": {
        "bytes": 0,
        "errors": 0
      }
    },
    "outputs": {
      "kafka": {
        "bytes_read": 1048670,
        "bytes_write": 43136887
      }
    },
    "pipeline": {
      "clients": 1,
      "events": {
        "active": 0,
        "dropped": 0,
        "failed": 0,
        "filtered": 10923,
        "published": 172067,
        "retry": 14,
        "total": 182990
      },
      "queue": {
        "acked": 172067
      }
    }
  },
  "registrar": {
    "states": {
      "cleanup": 3446,
      "current": 16409,
      "update": 182990
    },
    "writes": {
      "fail": 0,
      "success": 11718,
      "total": 11718
    }
  },
  "system": {
    "cpu": {
      "cores": 32
    },
    "load": {
      "1": 32.49,
      "15": 41.9,
      "5": 40.16,
      "norm": {
        "1": 1.0153,
        "15": 1.3094,
        "5": 1.255
      }
    }
  }
}
`
