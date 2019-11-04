/* eslint-disable func-names */
/* eslint-disable prefer-arrow-callback */
/* eslint-disable object-shorthand */
/* eslint quotes: ["error", "single", { "allowTemplateLiterals": true }] */

const deleteRepo = require('../commands/deleteRepo').deleteRepo;

let username = '';
let username2 = '';
const stringNotExisting = Math.random().toString(36).slice(2);
const stringExisting = 'test';


function genRepoNames(num) {
  const repos = [];
  for (let i = 0; i < num; i += 1) {
    repos.push(`${stringExisting}-${Math.random().toString(36).slice(2)}`);
  }
  return repos;
}


const numRepos = 3;
const repoNamesUser = genRepoNames(numRepos);
const repoNamesUser2 = genRepoNames(numRepos);
const repoKinds = ['files', 'workspace', 'programs'];

let maxWaitMs = 3000;
let waitMs = 1000;

const multiplier = process.env.NOG_TEST_WAITINGTIME_MULTIPLIER;
if (multiplier) {
  maxWaitMs *= multiplier;
  waitMs *= multiplier;
}


function makeRepoPublic(client, reponame) {
  client.url(`http://127.0.0.1:3000/${username}/${reponame}/files`);
  client
    .expect.element('.js-show-share-settings').to.be.visible.before(
       maxWaitMs);
  client
    .click('.js-show-share-settings')
    .expect.element('.js-show-share-settings').to.be.visible.before(
      maxWaitMs);
  client
    .click('.js-toggle-public')
    .expect.element('.js-toggle-public').to.be.selected.before(maxWaitMs);
  client
    .click('.js-show-share-settings')
    .expect.element('.js-toggle-public').to.be.not.present.before(maxWaitMs);
}


function createRepos(client, repoNames, kinds, makePublic) {
  for (let i = 0; i < repoNames.length; i += 1) {
    const type = `.t-type-${kinds[i % 3]}`;
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-new-repo').to.be.visible.before(maxWaitMs);

    client.click('.t-new-repo');
    client.expect.element('.t-create-repo').to.be.visible.before(maxWaitMs);
    client.expect.element(type).to.be.visible.before(maxWaitMs);
    client
      .click(type)
      .expect.element(type).to.be.selected.before(maxWaitMs);
    client
      .setValue('.t-set-repo-name', repoNames[i])
      .click('.t-create');

    if (type.indexOf('workspace') > -1) {
      client.expect.element('.t-workspace').to.be.visible.before(maxWaitMs);
    } else {
      client.expect.element('.t-files').to.be.visible.before(maxWaitMs);
    }

    if (makePublic) {
      makeRepoPublic(client, repoNames[i]);
    }
  }
}


function firstItemWithTextPresent(client, string) {
  client.url(`http://127.0.0.1:3000/${username2}/${string}/files`);
  client.expect.element('.t-files').to.be.present.before(maxWaitMs);
  client.url('http://127.0.0.1:3000/#recent');
  client.expect.element('.nog-list-item-name').to.be.present.before(
      maxWaitMs);

  let present = false;
  client.element('css selector', '.t-repos-tab-recent', function (list) {
    client.elementIdElements(list.value.ELEMENT, 'css selector',
    '.nog-list-item-name', function (elts) {
      client.elementIdText(elts.value[0].ELEMENT, function (text) {
        if (text.value.indexOf(string) > -1) {
          present = true;
        }
        const msg = `Expected first item with name "${string}" to be present.`;
        this.assert.ok(present === true, msg);
      });
    });
  });
}


function itemWithTextPresent(client, sel, string) {
  let present = false;
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt, idx, arr) {
      client.elementIdText(elt.ELEMENT, function (text) {
        for (let i = 0; i < string.length; i += 1) {
          if (text.value.indexOf(string[i]) > -1) {
            present = true;
          }
          if (idx + 1 === arr.length && i + 1 === string.length) {
            const msg = (
              `Expected item of <${sel}> with text "${string[i]}" ` +
              `to be present.`
            );
            this.assert.ok(present === true, msg);
          }
        }
      });
    });
  });
}


function itemWithTextNotPresent(client, sel, string) {
  let notPresent = true;
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt, idx, arr) {
      client.elementIdText(elt.ELEMENT, function (text) {
        for (let i = 0; i < string.length; i += 1) {
          if (text.status === 0) {
            if (text.value.indexOf(string[i]) > -1) {
              notPresent = false;
            }
          }
          if (idx + 1 === arr.length && i + 1 === string.length) {
            const msg = (
              `Expected item of <${sel}> with text "${string[i]}" to be ` +
              `NOT present.`
            );
            this.assert.ok(notPresent === true, msg);
          }
        }
      });
    });
  });
}


function toggleItemWithText(client, sel, string) {
//  let match = false;
  let repoName = '';
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt) {
      client.elementIdText(elt.ELEMENT, function (text) {
        if (text.status === 0 && text.value !== '') {
          let match = false;
          for (let i = 0; i < string.length; i += 1) {
            repoName = text.value.split('\n')[0];
            if (repoName.indexOf(string[i]) > -1) {
              match = true;
            }
          }
          if (match) {
            client.elementIdElements(elt.ELEMENT, 'class name',
            'fa-thumb-tack', function (pins) {
              pins.value.forEach(function (elt2) {
                client.elementIdDisplayed(elt2.ELEMENT, function (visible) {
                  const msg = (
                    `Expected pin of item "${repoName}" to be visible.`
                  );
                  this.assert.ok(visible.value === true, msg);
                });
              });
            });
          }
        }
      });
    });
  });
  client.elements('css selector', sel, function (elements) {
    elements.value.forEach(function (elt) {
      client.elementIdText(elt.ELEMENT, function (text) {
        if (text.status === 0 && text.value !== '') {
          let match = false;
          for (let i = 0; i < string.length; i += 1) {
            repoName = text.value.split('\n')[0];
            if (repoName.indexOf(string[i]) > -1) {
              match = true;
            }
          }
          if (match) {
            client.elementIdElements(elt.ELEMENT, 'class name',
            'fa-thumb-tack', function (pins) {
              pins.value.forEach(function (elt2) {
                client.elementIdClick(elt2.ELEMENT, function (clicked) {
                  const msg = `Toggled pin of item "${repoName}".`;
                  this.assert.ok(clicked.status === 0, msg);
                });
              });
            });
          }
        }
      });
    });
  });
}


function checkOrderByLastUpdate(client, id) {
  const dates = [];
  client.elementIdElements(id, 'css selector', '.t-repo-update',
  function (items) {
    this.assert.ok(items.value.length > 1, 'Expected more than 1 repo ' +
      'to check the order of the repo list.');
    for (let i = 0; i < items.value.length; i += 1) {
      client.elementIdText(items.value[i].ELEMENT, function (txt) {
        dates.push(txt.value.replace('Updated: ', ''));
        const len = dates.length;
        if (len > 1) {
          const date1 = new Date(dates[len - 1]);
          const date2 = new Date(dates[len - 2]);
          const wrongOrder = date1 > date2;
          this.assert.ok(wrongOrder === false, 'Expected list of ' +
              'repos to be sorted by date of last modification.');
        }
      });
    }
  });
}


function checkOrderByOwnerName(client, id) {
  const names = [];
  client.elementIdElements(id, 'css selector', '.nog-list-item-name',
  function (items) {
    this.assert.ok(items.value.length > 1, 'Expected more than 1 repo ' +
        'to check the order of the repo list.');
    for (let i = 0; i < items.value.length; i += 1) {
      client.elementIdText(items.value[i].ELEMENT, function (txt) {
        names.push(txt.value.replace(/\/.*$/, ''));
        const len = names.length;
        if (len > 1) {
          const name1 = names[len - 1];
          const name2 = names[len - 2];
          const wrongOrder = name1 < name2;
          this.assert.ok(wrongOrder === false, 'Expected list of ' +
              'repos to be sorted by owner name.');
        }
      });
    }
  });
}


function checkOrderByRepoName(client, id) {
  const names = [];
  client.elementIdElements(id, 'css selector', '.nog-list-item-name',
  function (items) {
    this.assert.ok(items.value.length > 1, 'Expected more than 1 repo ' +
        'to check the order of the repo list.');
    for (let i = 0; i < items.value.length; i += 1) {
      client.elementIdText(items.value[i].ELEMENT, function (txt) {
        names.push(txt.value.replace(/^.*\//, ''));
        const len = names.length;
        if (len > 1) {
          const name1 = names[len - 1];
          const name2 = names[len - 2];
          const wrongOrder = name1 < name2;
          this.assert.ok(wrongOrder === false, 'Expected list of ' +
              'repos to be sorted by repo name.');
        }
      });
    }
  });
}


function repoItemContainsAllElements(client) {
  client.elements('css selector', '.nog-repo-list-item',
  function (items) {
    items.value.forEach(function (elt) {
      client.elementIdDisplayed(elt.ELEMENT, function (visible) {
        if (visible.value) {
          client.elementIdElement(elt.ELEMENT, 'css selector',
          '.nog-list-item-name', function (name) {
            client.elementIdText(name.value.ELEMENT, function (text) {
              this.assert.ok(text.value !== '', 'Expected name of repo');
            });
          });
          client.elementIdElement(elt.ELEMENT, 'tag name', 'i',
          function (pin) {
            client.elementIdDisplayed(pin.value.ELEMENT,
            function (pinVisible) {
              this.assert.ok(pinVisible.value === true, 'Expected pin ' +
                  'in repo item.');
            });
          });
          client.elementIdElement(elt.ELEMENT, 'class name', 't-repo-update',
          function (update) {
            client.elementIdText(update.value.ELEMENT, function (txt) {
              const str = txt.value;
              str.replace('Updated: ', '');
              const date = new Date(str);
              const valid =
                  Object.prototype.toString.call(date) === '[object Date]';
              this.assert.ok(valid === true, 'Expected date of last repo ' +
                  'modification.');
            });
          });
          client.elementIdElement(elt.ELEMENT, 'tag name', 'img',
          function (img) {
            this.assert.ok(img.status === 0, 'Expected icon in repo item');
          });
        }
      });
    });
  });
}


// `setLongString` is a workaround for `setValue` to ensure that the strings
// are sent to the input forms correctly.  `setValue` sometimes does not send
// all characters to the input.  For example, 'alovelace' results in 'alovelce'
// and filter test fail due to wrong filter results.  `setLongString` sends the
// string character by character.
// This issue was discussed here:
// https://github.com/nightwatchjs/nightwatch/issues/983
function setLongString(client, using, sel, string) {
  client.elements(using, sel, function (elements) {
    elements.value.forEach(function (elt) {
      for (const c of string.split('')) {
        client.elementIdValue(elt.ELEMENT, c);
      }
    });
  });
}


module.exports = {
  before(client) {
    console.log(
      `The tests run with 'maxWaitMs' ${maxWaitMs} ms and 'waitMs' = ` +
      `${waitMs} ms.`
    );
    client
      .url('http://127.0.0.1:3000')
      .expect.element('.t-login-user').to.be.visible.before(waitMs);
    client
      .click('.t-login-user')
      .pause(waitMs);
    client.getText('.t-current-user', function (result) {
      username = result.value;
      createRepos(client, repoNamesUser, repoKinds, true);
    });
    client
      .click('.t-logout')
      .pause(waitMs);

    client
      .click('.t-login-user2')
      .pause(waitMs);
    client.getText('.t-current-user', function (result) {
      username2 = result.value;
      createRepos(client, repoNamesUser2, repoKinds, false);
    });

    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
  },

  after(client) {
    for (let i = 0; i < repoNamesUser2.length; i += 1) {
      deleteRepo(client, username2, repoNamesUser2[i], maxWaitMs);
    }
    client
      .click('.t-logout')
      .pause(waitMs);
    client
      .click('.t-login-user')
      .pause(waitMs);
    for (let i = 0; i < repoNamesUser.length; i += 1) {
      deleteRepo(client, username, repoNamesUser[i], maxWaitMs);
    }
    client.end();
  },

  '[repo lists tests] Logged in': function (client) {
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-current-user').text.to.contain(username);
  },

  '[repo lists tests] Own repos are listed in tab "own"': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
  },

  '[repo lists tests] Shared repos are listed in tab "shared"':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.t-repos-tab-shared')
      .to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] All repos are listed in tab "all"': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.t-repos-tab-all').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
  },

  '[repo lists tests] Pinned repos appear in tab "favorites"':
  function (client) {
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    client.expect.element('.t-repos-tab-favorites')
      .to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "own": only own': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "own": only shared': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "own": existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', stringExisting);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
  },

  '[repo lists tests] Filtering in tab "own": not existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.t-repos-tab-own').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos',
      stringNotExisting);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
  },

  '[repo lists tests] Filtering in tab "shared": only shared':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.t-repos-tab-shared')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "shared": only own': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.t-repos-tab-shared')
      .to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "shared": existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.t-repos-tab-shared')
      .to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', stringExisting);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "shared": not existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.t-repos-tab-shared')
      .to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos',
      stringNotExisting);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "all": only own': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.t-repos-tab-all').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "all": only shared': function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.t-repos-tab-all').to.be.visible.before(maxWaitMs);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', username);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "all": existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.t-repos-tab-all').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos', stringExisting);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "all": not existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.t-repos-tab-all').to.be.visible.before(maxWaitMs);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    setLongString(client, 'css selector', '.js-filter-repos',
      stringNotExisting);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "favorites": only own':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    setLongString(client, 'css selector', '.js-filter-repos', username2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "favorites": only shared':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    setLongString(client, 'css selector', '.js-filter-repos', username);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "favorites": existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    setLongString(client, 'css selector', '.js-filter-repos', stringExisting);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Filtering in tab "favorites": not existing string':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextPresent(client, '.nog-list-item-name', repoNamesUser);
    client.expect.element('.js-filter-repos').to.be.visible.before(maxWaitMs);
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    setLongString(client, 'css selector', '.js-filter-repos',
      stringNotExisting);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser2);
    itemWithTextNotPresent(client, '.nog-list-item-name', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Repo items are displayed completely after a while':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    repoItemContainsAllElements(client, '.nog-reo-list-item');
  },

  '[repo lists tests] Repos in tab "all" are sorted by recent update':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="recent"]')
      .keys('Enter');
    client.element('id', 'all', function (list) {
      checkOrderByLastUpdate(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "own" are sorted by recent update':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="recent"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-own').to.be.present.before(maxWaitMs);
    client.element('id', 'own', function (list) {
      checkOrderByLastUpdate(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "shared" are sorted by recent update':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="recent"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-shared')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'shared', function (list) {
      checkOrderByLastUpdate(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "favorites" are sorted by recent update':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="recent"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-favorites')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'favorites', function (list) {
      checkOrderByLastUpdate(client, list.value.ELEMENT);
    });
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Repos in tab "all" are sorted by owner name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="owner"]')
      .keys('Enter');
    client.element('id', 'all', function (list) {
      checkOrderByOwnerName(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "shared" are sorted by owner name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="owner"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-shared')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'shared', function (list) {
      checkOrderByOwnerName(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "favorites" are sorted by owner name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="owner"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-favorites')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'favorites', function (list) {
      checkOrderByOwnerName(client, list.value.ELEMENT);
    });
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Repos in tab "all" are sorted by repo name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="repo"]')
      .keys('Enter');
    client.element('id', 'all', function (list) {
      checkOrderByRepoName(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "shared" are sorted by repo name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-shared').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-shared');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="repo"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-shared')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'shared', function (list) {
      checkOrderByRepoName(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "own" are sorted by repo name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-own').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-own');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="repo"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-own').to.be.present.before(maxWaitMs);
    client.element('id', 'own', function (list) {
      checkOrderByRepoName(client, list.value.ELEMENT);
    });
  },

  '[repo lists tests] Repos in tab "favorites" are sorted by repo name':
  function (client) {
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-favorites').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-favorites');
    client.expect.element('.js-select-sort').to.be.visible.before(maxWaitMs);
    client
      .click('select[id="sort"]')
      .click('option[name="repo"]')
      .keys('Enter');
    client.expect.element('.t-repos-tab-favorites')
      .to.be.present.before(maxWaitMs);
    client.element('id', 'favorites', function (list) {
      checkOrderByRepoName(client, list.value.ELEMENT);
    });
    client.url('http://127.0.0.1:3000/');
    client.expect.element('.t-tab-all').to.be.visible.before(maxWaitMs);
    client.click('.t-tab-all');
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser2);
    toggleItemWithText(client, '.nog-repo-list-item', repoNamesUser);
  },

  '[repo lists tests] Recently visited repos are inserted on top of "recent"':
  function (client) {
    for (let i = 0; i < repoNamesUser2.length; i += 1) {
      firstItemWithTextPresent(client, repoNamesUser2[i]);
    }
  },
};
