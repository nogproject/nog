import { makePubName } from './fso-pubsub.js';

// Poll GRPC backends  with caching in Mongo:
import { publishRepoFunc } from './fso-pub-repo.js';

// The `publisher` is usually `Meteor` or a testing mock.
function createPublications({
  testAccess, namespace, publisher, repos, openRepo, broadcast,
}) {
  function defPub(name, fn) {
    publisher.publish(makePubName(namespace, name), fn);
  }

  defPub('repo', publishRepoFunc({
    testAccess, repos, openRepo, broadcast,
  }));
}

export {
  createPublications,
};
