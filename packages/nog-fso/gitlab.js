import { check, Match } from 'meteor/check';

function createGitlabLocation(config) {
  check(config, Match.ObjectIncluding({
    ui: String,
  }));

  return {
    config,

    projectUiUrl(projectPath) {
      const { ui } = this.config;
      return `${ui}/${projectPath}`;
    },
  };
}

export {
  createGitlabLocation,
};
