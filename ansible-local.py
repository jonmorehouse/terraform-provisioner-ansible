#!/usr/bin/env python
# vim:syntax=python

import argparse
import collections
import json
import os
import os.path

from ansible.vars import VariableManager
from ansible.parsing.dataloader import DataLoader
from ansible.inventory import Inventory
from ansible.inventory import Group
from ansible.playbook import Playbook
from ansible.executor.task_queue_manager import TaskQueueManager


OptionsClass = collections.namedtuple('Options', ('connection', 'module_path', 'forks', 'remote_user',
                                                  'private_key_file', 'ssh_common_args', 'ssh_extra_args',
                                                  'sftp_extra_args', 'scp_extra_args', 'become', 'become_method',
                                                  'become_user', 'verbosity', 'check'))


def build_inventory(loader, variable_manager, group_names, playbook_basedir):
    inventory = Inventory(loader=loader, variable_manager=variable_manager, host_list=['localhost'])

    # because we just pass in a "host list" which isn't a real inventory file,
    # we explicitly have to add all of the desired groups to the inventory. By
    # default an "all" group is created whenever a new inventory is created
    for group_name in group_names:
        if not inventory.get_group(group_name):
            inventory.add_group(Group(group_name))

    # because we are explicitly adding groups, we also need to make sure that a
    # playbook basedir is set so that `group_vars` can be loaded from the
    # correct directory.
    inventory.set_playbook_basedir(playbook_basedir)

    # for each group specified, ensure that the inventory's host (localhost) is
    # explicitly in the group.
    for group_name in group_names:
        group = inventory.get_group(group_name)
        if group.get_hosts():
            continue

        for host in inventory.get_hosts():
            group.add_host(host)

    return inventory


def build_plays(loader, variable_manager, playbook_path, plays=[], hosts=[], roles=[]):
    playbook = Playbook.load(playbook_path, variable_manager, loader)
    _plays = []

    for play in playbook.get_plays():
        if play.get_name() in plays:
            _plays.append(play)
            continue

        if play._ds['hosts'] in hosts:
            _plays.append(play)
            continue

        for piece in play._ds['hosts']:
            if piece in hosts:
                _plays.append(play)
                break

        for role in play.get_roles():
            if role.get_name() in roles:
                _plays.append(play)

    return _plays


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='ansible-local script for running ansible against localhost')
    string_to_list = lambda x: x.split(',')
    json_parser = lambda x: json.loads(x)

    parser.add_argument('--playbook', help='full filepath of the playbook', required=True)
    parser.add_argument('--extra-vars', help='json encoded string with extra vars', required=False, type=json_parser)
    parser.add_argument('--groups', help='ansible groups', required=False, type=string_to_list)
    parser.add_argument('--plays', help='named plays to run', required=False, type=string_to_list)
    parser.add_argument('--hosts', help='host groups to run', required=False, type=string_to_list)
    parser.add_argument('--roles', help='explicit list of roles to apply', required=False, type=string_to_list)
    args = parser.parse_args()

    # clean up arguments that were optional or semioptional
    assert args.plays or args.hosts or args.roles, "at a minimum, a list of roles, groups or hosts must be specified"
    if not args.plays:
        args.plays = []
    if not args.hosts:
        args.hosts = []
    if not args.roles:
        args.roles = []

    if not args.extra_vars:
        args.extra_vars = {}

    playbook_basedir = os.path.dirname(os.path.abspath(args.playbook))

    options = OptionsClass(connection='local',
                           module_path=playbook_basedir,
                           forks=100,
                           remote_user=None,
                           private_key_file=None,
                           ssh_common_args=None,
                           ssh_extra_args=None,
                           sftp_extra_args=None,
                           scp_extra_args=None,
                           become=None,
                           become_method='sudo',
                           become_user='root',
                           verbosity=None,
                           check=False)

    loader = DataLoader()
    variable_manager = VariableManager()
    variable_manager.extra_vars.update(args.extra_vars)

    # we apply an exhaustive list of groups to the host that is created in the
    # inventory, simply to ensure that we resolve the correct hosts for each
    # play. This has the unfortunate side effect that if someone has a
    # group_var that shadows the name of a play, it could be applied unexpectedly.
    # TODO: look up plays by name and fetch the host_groups explicitly to be added.
    inventory_host_groups = args.groups + args.hosts + args.plays
    inventory = build_inventory(loader, variable_manager, inventory_host_groups, playbook_basedir)

    variable_manager.set_inventory(inventory)
    plays = build_plays(loader, variable_manager, args.playbook, plays=args.plays, hosts=args.hosts, roles=args.roles)

    tqm = TaskQueueManager(
        inventory=inventory,
        variable_manager=variable_manager,
        loader=loader,
        passwords=None,
        options=options,
        stdout_callback='default',
        run_tree=False,
    )

    for play in plays:
        tqm.run(play)
