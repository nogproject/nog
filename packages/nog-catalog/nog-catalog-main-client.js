import { _ } from 'meteor/underscore';
import { defCatalogMethods } from './nog-catalog-methods.js';
import { Mongo } from 'meteor/mongo';
import { makeCollName, makePubName } from './nog-catalog-common.js';


function createVolumeRegistry() {
  return {
    volumes: {},
    metaKeyLists: {},

    getCollection(active, volName) {
      // Access the metaKeyMap in `transform()` by name via the catalogManager
      // instance variable, so that potential updates of the map will be picked
      // up.

      const { metaKeyLists } = this;
      metaKeyLists[volName] = [...active.metaKeys];

      let coll = this.volumes[volName];
      if (coll) {
        return coll;
      }

      coll = new Mongo.Collection(volName, {
        transform(doc) {
          const metaKeys = metaKeyLists[volName];
          if (!metaKeys) {
            return doc;
          }

          // The encoded fields should be moved to a nested object `m.` instead
          // to avoid regex matching on all fields.

          const meta = {};
          for (const [k, v] of _.pairs(doc.m)) {
            const idx = Number(k.slice(1));
            const mk = metaKeys[idx] || k;
            meta[mk] = v;
          }

          return { ...doc, meta };
        },
      });
      coll.stats = new Mongo.Collection(`${volName}.stats`);
      this.volumes[volName] = coll;

      return coll;
    },
  };
}


function createCatalogClientModule({
  namespace, testAccess,
}) {
  const catalogs = new Mongo.Collection(makeCollName(namespace, 'catalogs'));

  const mod = {
    testAccess,
    catalogs,
    volumeRegistry: createVolumeRegistry(),

    subscribeCatalog(manager, catName) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalog'), catName,
      );
      return handle;
    },
    subscribeCatalogHitCount(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogHitCount'), opts,
      );
      return handle;
    },
    subscribeCatalogVolume(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogVolume'), opts,
      );
      return handle;
    },
    subscribeCatalogVolumeStats(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogVolumeStats'), opts,
      );
      return handle;
    },

    subscribeCatalogFso(manager, catName) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogFso'), catName,
      );
      return handle;
    },
    subscribeCatalogHitCountFso(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogHitCountFso'), opts,
      );
      return handle;
    },
    subscribeCatalogVolumeFso(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogVolumeFso'), opts,
      );
      return handle;
    },
    subscribeCatalogVolumeStatsFso(manager, opts) {
      const handle = manager.subscribe(
        makePubName(namespace, 'catalogVolumeStatsFso'), opts,
      );
      return handle;
    },
  };

  defCatalogMethods({ namespace, mod });

  return mod;
}


export { createCatalogClientModule };
